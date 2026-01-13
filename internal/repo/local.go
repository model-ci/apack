package repo

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/internal/task"
	"github.com/model-ci/apack/internal/utils"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/model-ci/apack/pkg/progress"
	modelspec "github.com/modelpack/model-spec/specs-go/v1"
	"github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"oras.land/oras-go/v2/registry"
)

type local struct {
	db       layerdb.DB
	dbRaw    layerdb.RAW
	snapPath string
}

func New(path string, db layerdb.DB) (distribution.Distribution, error) {
	dbRaw, err := db.Raw(nil)
	if err != nil {
		return nil, err
	}
	return &local{
		db:       db,
		dbRaw:    dbRaw,
		snapPath: filepath.Join(path, "snapshots"),
	}, nil
}

func (l *local) Pull(ctx context.Context, remote registry.Repository, ref registry.Reference, plog *progress.Logger, opts *distribution.Options) (oci.Descriptor, error) {
	desc, err := remote.Resolve(ctx, ref.Reference)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	manifest, err := GetManifest(ctx, remote, desc)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	config, err := GetConfig(ctx, remote, manifest.Config)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	toPull := manifest.Layers
	toPull = append(toPull, manifest.Config, desc)
	sem := semaphore.NewWeighted(int64(opts.Concurrency))
	pullProgress := progress.NewProgress()
	errs, errCtx := errgroup.WithContext(ctx)
	fmtErr := func(desc oci.Descriptor, err error) error {
		if err == nil {
			return nil
		}
		return fmt.Errorf("failed to get %+v layer: %w", desc, err)
	}

	var semErr error
	pulledDigests := map[string]bool{}
	layers := len(manifest.Layers)
	for index, pull := range toPull {
		pullDesc := pull
		pullDescIndex := index

		digestStr := pullDesc.Digest.String()
		if pulledDigests[digestStr] {
			continue
		}
		pulledDigests[digestStr] = true
		if err := sem.Acquire(errCtx, 1); err != nil {
			semErr = err
			break
		}

		pw := progress.NewProgressWriter(plog, task.ActionPull, pullDesc.Digest.Encoded()[:12], pullDesc.Size)
		pullProgress.Add(pw)

		errs.Go(func() error {
			defer sem.Release(1)
			diffid := digest.Digest("")
			if pullDescIndex < layers {
				diffid = config.RootFS.DiffIDs[pullDescIndex]
			}
			return fmtErr(pullDesc, l.pullLayer(errCtx, remote, pullDesc, diffid, ref.String(), pw))
		})
	}
	if err := errs.Wait(); err != nil {
		return oci.DescriptorEmptyJSON, err
	}
	if semErr != nil {
		return oci.DescriptorEmptyJSON, fmt.Errorf("failed to acquire lock: %w", semErr)
	}

	if !pullProgress.CheckAllCompleted() {
		return oci.DescriptorEmptyJSON, fmt.Errorf("failed to pull all layers")
	}

	return desc, l.db.Index(ctx, ref.String(), desc)
}

func (l *local) pullLayer(ctx context.Context, remote registry.Repository, desc oci.Descriptor, diffid digest.Digest, ref string, pw *progress.ProgressWriter) error {
	if exists, err := l.db.Exists(ctx, desc); err != nil {
		return fmt.Errorf("failed to check local storage: %w", err)
	} else if exists {
		pw.MarkCompleted()
		return nil
	}

	blob, err := remote.Fetch(ctx, desc)
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	pmt := layerdb.ParseMediaType(desc.MediaType)
	if pmt.IsTar {
		err := l.extract(ctx, desc, diffid, l.Snappath(ctx, ref), blob, pw)
		if err != nil {
			return err
		}
	} else {
		if err := l.db.Set(ctx, desc, blob); err != nil {
			return err
		}
	}

	pw.MarkCompleted()

	return nil
}

func (l *local) Push(ctx context.Context, remote registry.Repository, ref registry.Reference, plog *progress.Logger, opts *distribution.Options) (oci.Descriptor, error) {
	refs := ref.String()
	manifest, desc, err := l.db.Manifest(ctx, refs)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	toPush := []oci.Descriptor{manifest.Config}
	toPush = append(toPush, manifest.Layers...)
	sem := semaphore.NewWeighted(int64(opts.Concurrency))
	pushProgress := progress.NewProgress()
	errs, errCtx := errgroup.WithContext(ctx)
	fmtErr := func(desc oci.Descriptor, err error) error {
		if err == nil {
			return nil
		}
		return fmt.Errorf("failed to push %s layer: %w", desc.MediaType, err)
	}

	var semErr error
	pushedDigests := map[string]bool{}
	for _, push := range toPush {
		pushDesc := push
		digest := pushDesc.Digest.String()
		if pushedDigests[digest] {
			continue
		}
		pushedDigests[digest] = true
		if err := sem.Acquire(errCtx, 1); err != nil {
			semErr = err
			break
		}

		pw := progress.NewProgressWriter(plog, task.ActionPush, pushDesc.Digest.Encoded()[:12], pushDesc.Size)
		pushProgress.Add(pw)

		errs.Go(func() error {
			defer sem.Release(1)
			return fmtErr(pushDesc, l.pushLayer(errCtx, refs, remote, pushDesc, pw))
		})
	}
	if err := errs.Wait(); err != nil {
		return oci.DescriptorEmptyJSON, err
	}
	if semErr != nil {
		return oci.DescriptorEmptyJSON, fmt.Errorf("failed to acquire lock: %w", semErr)
	}

	if !pushProgress.CheckAllCompleted() {
		return oci.DescriptorEmptyJSON, fmt.Errorf("failed to push all layers")
	}

	mf := &layerdb.Manifest{Manifest: manifest}
	manifestBytes, err := mf.MarshalJSON()
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	err = remote.PushReference(ctx, desc, bytes.NewReader(manifestBytes), ref.String())
	if err != nil {
		log.Logger.Debugf("failed to push manifest: %s", err)
		return oci.DescriptorEmptyJSON, err
	}

	return desc, nil
}

func (l *local) pushLayer(ctx context.Context, ref string, remote registry.Repository, desc oci.Descriptor, pw *progress.ProgressWriter) error {
	if exists, err := remote.Exists(ctx, desc); err != nil {
		return fmt.Errorf("failed to check local storage: %w", err)
	} else if exists {
		pw.MarkCompleted()
		return nil
	}

	var err, cerr error
	var content io.ReadCloser

	filename, ok := desc.Annotations[modelspec.AnnotationFilepath]
	if ok {
		snapref := l.Snaplink(ctx, ref)
		snapfile := filepath.Join(snapref, filename)
		f, err := os.Open(snapfile)
		if err != nil {
			return err
		}

		fi, err := f.Stat()
		if err != nil {
			return err
		}

		content, cerr = l.db.Contenting(ctx, desc.MediaType, layerdb.NewFile(fi, f), nil)
	} else {
		content, err = l.db.Read(ctx, desc)
		if err != nil {
			log.Logger.Debugf("failed to read layer: %+v, %s", desc, err)
			return err
		}
	}

	progressReader := io.TeeReader(content, pw)
	err = remote.Push(ctx, desc, progressReader)
	if err != nil {
		log.Logger.Debugf("failed to push layer: %+v, %s", desc, err)
		return err
	}

	if cerr != nil {
		return cerr
	}

	pw.MarkCompleted()

	return nil
}

func (l *local) Bundle(ctx context.Context, mf distribution.Makefile, plog *progress.Logger, opts *distribution.Options) (oci.Descriptor, error) {
	config := &layerdb.Config{}
	var layersMu sync.Mutex
	var layers []oci.Descriptor

	contents, err := mf.Contents()
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	log.Logger.Debugf("start bundle for: %s", mf.Reference())

	sem := semaphore.NewWeighted(int64(opts.Concurrency))
	bundleProgress := progress.NewProgress()
	errs, errCtx := errgroup.WithContext(ctx)
	fmtErr := func(mediaType string, err error) error {
		if err == nil {
			return nil
		}
		return fmt.Errorf("failed to bundle %s layer: %w", mediaType, err)
	}

	snapRef, err := l.Snapshot(ctx, mf.Reference())
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	var semErr error
	for _, content := range contents {
		c := content
		if c.Path == "" {
			log.Logger.Warnf("bundle file path is null: %s", c.MediaType)
			continue
		}

		if err := sem.Acquire(errCtx, 1); err != nil {
			semErr = err
			break
		}

		pw := progress.NewProgressWriter(plog, task.ActionBundle, c.Metadata.Name, c.Metadata.Size)
		bundleProgress.Add(pw)

		errs.Go(func() error {
			defer sem.Release(1)
			return fmtErr(c.MediaType, l.bundleLayer(errCtx, &c, snapRef, &layers, &layersMu, config, pw))
		})
	}

	if err := errs.Wait(); err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	if semErr != nil {
		return oci.DescriptorEmptyJSON, fmt.Errorf("failed to acquire lock: %w", semErr)
	}

	if !bundleProgress.CheckAllCompleted() {
		return oci.DescriptorEmptyJSON, fmt.Errorf("failed to bundle all layers")
	}

	config.Created = utils.TimePtr(time.Now())

	configBytes, err := config.MarshalJSON()
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	artifactBytes, err := mf.Artifact().MarshalJSON()
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	configDesc := layerdb.ConfigDesc(configBytes, mf.ConfigMediaType())
	configDesc.ArtifactType = mf.ArtifactConfigType()
	configDesc.Data = artifactBytes
	err = l.db.Write(ctx, configDesc, layerdb.BytesToReadCloser(configBytes), nil)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	manifestDesc, manifestBytes, err := layerdb.ManifestDesc(configDesc, oci.Descriptor{}, layers, mf.ManifestMediaType())
	manifestDesc.ArtifactType = mf.ArtifactManifestType()
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	err = l.db.Write(ctx, manifestDesc, layerdb.BytesToReadCloser(manifestBytes), nil)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	ref := mf.Reference()
	if ref == "" {
		ref = manifestDesc.Digest.Encoded()
	}

	return manifestDesc, l.db.Index(ctx, ref, manifestDesc)
}

func (l *local) BundleOnce(ctx context.Context, content *distribution.Content, snapref string, layers *[]oci.Descriptor, layersMu *sync.Mutex, config *layerdb.Config, pw *progress.ProgressWriter) error {
	return l.bundleLayer(ctx, content, snapref, layers, layersMu, config, pw)
}

func (l *local) bundleLayer(ctx context.Context, content *distribution.Content, snapref string, layers *[]oci.Descriptor, layersMu *sync.Mutex, config *layerdb.Config, pw *progress.ProgressWriter) error {
	filename := filepath.Base(content.Path)
	snapname := filepath.Join(snapref, filename)

	snap, err := os.Create(snapname)
	if err != nil {
		return err
	}

	layer, digest, err := l.db.Layering(ctx, content.MediaType, content, snap, pw)
	if err != nil {
		return err
	}

	layer.ArtifactType = content.ArtifactType
	layer.Annotations[modelspec.AnnotationFilepath] = content.Path

	layersMu.Lock()
	*layers = append(*layers, layer)
	config.RootFS.DiffIDs = append(config.RootFS.DiffIDs, digest)
	layersMu.Unlock()

	config.RootFS.Type = layerdb.TypeLayers

	err = l.db.Set(ctx, layer, io.NopCloser(bytes.NewReader([]byte(snapname))))
	if err != nil {
		return err
	}

	pw.MarkCompleted()

	return nil
}

func (l *local) Extract(ctx context.Context, dir string, reference string, plog *progress.Logger, opts *distribution.Options) error {
	manifest, _, err := l.db.Manifest(ctx, reference)
	if err != nil {
		return err
	}

	data, err := l.db.Read(ctx, manifest.Config)
	if err != nil {
		return err
	}

	config := layerdb.Config{}
	if err = config.UnmarshalJSONFromIO(data); err != nil {
		return err
	}

	sem := semaphore.NewWeighted(int64(opts.Concurrency))
	extractProgress := progress.NewProgress()
	errs, errCtx := errgroup.WithContext(ctx)
	fmtErr := func(desc oci.Descriptor, err error) error {
		if err == nil {
			return nil
		}
		return fmt.Errorf("failed to extract %s layer: %w", desc.MediaType, err)
	}
	var semErr error
	extractedDigests := map[string]bool{}

	for i, layer := range manifest.Layers {
		layerDesc := layer
		layerIndex := i
		digest := layerDesc.Digest.String()
		if extractedDigests[digest] {
			continue
		}
		extractedDigests[digest] = true
		if err := sem.Acquire(errCtx, 1); err != nil {
			semErr = err
			break
		}

		pw := progress.NewProgressWriter(plog, task.ActionExtract, layerDesc.Digest.Encoded()[:12], layerDesc.Size)
		extractProgress.Add(pw)

		errs.Go(func() error {
			defer sem.Release(1)
			return fmtErr(layerDesc,
				l.extract(errCtx, layerDesc, config.RootFS.DiffIDs[layerIndex], dir, nil, pw))
		})
	}

	if err := errs.Wait(); err != nil {
		return err
	}

	if semErr != nil {
		return fmt.Errorf("failed to acquire lock: %w", semErr)
	}

	if !extractProgress.CheckAllCompleted() {
		return fmt.Errorf("failed to extract all layers")
	}

	return nil
}

func (l *local) extract(ctx context.Context, desc oci.Descriptor, diffid digest.Digest, dir string, content io.ReadCloser, pw *progress.ProgressWriter) error {
	path := desc.Annotations[modelspec.AnnotationFilepath]
	cont, err := l.db.Content(ctx, desc, diffid, content, true)
	if err != nil {
		return err
	}
	return l.extractLayer(ctx, filepath.Join(dir, path), cont, pw)
}

func (l *local) extractLayer(ctx context.Context, path string, src io.Reader, pw *progress.ProgressWriter) error {
	progressReader := io.TeeReader(src, pw)

	tarReader := tar.NewReader(progressReader)

	header, err := tarReader.Next()
	if err != nil {
		return fmt.Errorf("failed to read tar header: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, tarReader); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	pw.MarkCompleted()

	return nil
}

func (l *local) Artifacts(ctx context.Context) ([]spec.Artifact, error) {
	return l.artifacts(ctx, "" /* reference */)
}

func (l *local) Artifact(ctx context.Context, reference string) (spec.Artifact, error) {
	items, err := l.artifacts(ctx, reference)
	if err != nil {
		return spec.Artifact{}, err
	}
	if len(items) == 0 {
		return spec.Artifact{}, fmt.Errorf("no artifact found for reference: %s", reference)
	}
	return items[0], nil
}

func (l *local) artifacts(ctx context.Context, reference string) ([]spec.Artifact, error) {
	var artifacts []spec.Artifact

	if reference != "" {
		manifest, _, err := l.db.Manifest(ctx, reference)
		if err != nil {
			return nil, err
		}
		artifact, err := l.buildAritifact(ctx, reference, manifest)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
		return artifacts, nil
	}

	manifests, refs, err := l.db.Manifests(ctx)
	if err != nil {
		return nil, err
	}

	for i, ref := range refs {
		manifest := manifests[i]
		artifact, err := l.buildAritifact(ctx, ref, manifest)
		if err != nil {
			return nil, err
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

func (l *local) buildAritifact(ctx context.Context, ref string, mf oci.Manifest) (spec.Artifact, error) {
	a := spec.Artifact{}
	if err := a.UnmarshalJSON(mf.Config.Data); err != nil {
		return spec.Artifact{}, err
	}

	size := 0
	for _, layer := range mf.Layers {
		size += int(layer.Size)
	}

	_, desc, err := l.db.Manifest(ctx, ref)
	if err != nil {
		return spec.Artifact{}, err
	}

	a.Package.Size = int64(size)
	a.Package.Digest = desc.Digest.Encoded()
	a.Package.Reference = ref

	return a, nil
}

func (l *local) Manifest(ctx context.Context, reference string) (oci.Manifest, oci.Descriptor, error) {
	return l.db.Manifest(ctx, reference)
}

func (l *local) State(ctx context.Context, reference string, desc oci.Descriptor) error {
	ldesc := &layerdb.Descriptor{Descriptor: desc}
	ldescBytes, err := ldesc.MarshalJSON()
	if err != nil {
		return err
	}
	return l.dbRaw.Put(ctx, reference, string(ldescBytes))
}

func (l *local) Statuses(ctx context.Context) ([]oci.Descriptor, error) {
	var descs []oci.Descriptor

	err := l.dbRaw.ForEach(ctx, func(key, value []byte) error {
		ldesc := &layerdb.Descriptor{}
		if err := ldesc.UnmarshalJSON(value); err != nil {
			return err
		}
		descs = append(descs, ldesc.Descriptor)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return descs, nil
}

func (l *local) Snappath(ctx context.Context, reference string) string {
	return filepath.Join(l.snapPath, reference)
}

func (l *local) Snaplink(ctx context.Context, reference string) string {
	path := filepath.Join(l.snapPath, reference)
	if !IsLinkFileExist(path) {
		return path
	}
	refpath, _ := ReadLinkFile(path)
	return refpath
}

func (l *local) Snapshot(ctx context.Context, reference string) (string, error) {
	dir := filepath.Join(l.snapPath, reference)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}
	return dir, l.db.Snap(ctx, reference)
}

func (l *local) Tag(ctx context.Context, ref1, ref2 registry.Reference) error {
	err := l.db.Copy(ctx, ref1.String(), ref2.String())
	if err != nil {
		return err
	}

	ref1path := l.Snappath(ctx, ref1.String())
	ref2path := l.Snappath(ctx, ref2.String())

	if IsLinkFileExist(ref2path) {
		// This ref has already been tagged
		return nil
	}

	if IsLinkFileExist(ref1path) && !IsLinkFileExist(ref2path) {
		// This means that ref1 is a tagged ref,
		// so it is necessary to copy the link of
		// ref1 to ref2 instead of directly giving
		// the ref of ref1 to ref2
		ref, err := ReadLinkFile(ref1path)
		if err != nil {
			return err
		}
		return WriteLinkFile(ref2path, ref)
	}

	return WriteLinkFile(ref2path, ref1path)
}

func (l *local) Config(ctx context.Context, desc oci.Descriptor) (layerdb.Config, error) {
	content, err := l.db.Read(ctx, desc)
	if err != nil {
		return layerdb.Config{}, err
	}
	c := &layerdb.Config{}
	return *c, c.UnmarshalJSONFromIO(content)
}

func (l *local) Remove(ctx context.Context, reference string) error {
	return l.db.Delete(ctx, reference)
}
