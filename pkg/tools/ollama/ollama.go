package ollama

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/pkg/utils"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/model-ci/apack/pkg/progress"
	"github.com/model-ci/apack/pkg/tools"
	modelspec "github.com/modelpack/model-spec/specs-go/v1"
	"github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	Name = "ollama"
)

func init() {
	tools.Register(Name, NewOllama)
}

type Ollama struct {
	username    string
	password    string
	concurrency int
	db          layerdb.DB
	dstb        distribution.Distribution
}

func NewOllama(user, password string, concurrency int, db layerdb.DB, dstb distribution.Distribution) (tools.Tool, error) {
	return &Ollama{
		concurrency: concurrency,
		db:          db,
		dstb:        dstb,
	}, nil
}

func (o *Ollama) Fetch(ctx context.Context, reference string, path string, plog *progress.Logger) (oci.Descriptor, error) {
	repo, err := NewRepository(reference)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	desc, err := repo.Resolve(ctx, repo.Reference.Reference)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	manifest, err := getManifest(ctx, repo.Repository, desc)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	conf, err := getConfig(ctx, repo.Repository, manifest.Config)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	toFetch := manifest.Layers
	toFetch = append(toFetch, manifest.Config, desc)
	sem := semaphore.NewWeighted(int64(o.concurrency))
	fetchProgress := progress.NewProgress()
	errs, errCtx := errgroup.WithContext(ctx)
	fmtErr := func(desc oci.Descriptor, err error) error {
		if err != nil {
			return fmt.Errorf("failed to get %+v layer: %w", desc, err)
		}
		return nil
	}

	config := &layerdb.Config{}
	var layersMu sync.Mutex
	var layers []oci.Descriptor

	pkg := spec.Package{
		Name: fmt.Sprintf("%s-%s-%s-%s.%s",
			conf.ModelFamily, repo.Reference.Reference, conf.OS, conf.Architecture, conf.ModelFormat),
		Workspace: Name,
		Reference: reference,
		Size:      desc.Size,
		Digest:    desc.Digest.String(),
	}

	var semErr error
	fetchedDigests := map[string]bool{}
	layerCount := len(manifest.Layers)

	snapDiff, err := o.dstb.Snapdiff(ctx)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	for index, fetch := range toFetch {
		fetchDesc := fetch
		fetchDescIndex := index

		digestStr := fetchDesc.Digest.String()
		if fetchedDigests[digestStr] {
			continue
		}
		fetchedDigests[digestStr] = true
		if err := sem.Acquire(errCtx, 1); err != nil {
			semErr = err
			break
		}

		pw := progress.NewProgressWriter(plog, tools.ActionFetch, fetchDesc.Digest.Encoded()[:12], fetchDesc.Size)
		fetchProgress.Add(pw)

		errs.Go(func() error {
			defer sem.Release(1)
			diffid := digest.Digest("")
			if fetchDescIndex < layerCount {
				diffid = digest.Digest(conf.RootFS.DiffIDs[fetchDescIndex])
				fetchDesc.Annotations = map[string]string{}
				fetchDesc.Annotations[modelspec.AnnotationFilepath] = getFilename(fetchDesc.MediaType, fetch.Digest.Encoded(), fetch.Size, conf, &pkg)
			}
			return fmtErr(fetchDesc, o.fetchLayer(errCtx, repo, fetchDesc, diffid, reference, path, snapDiff, &layers, &layersMu, config, pw, fetchProgress, plog))
		})
	}

	if err := errs.Wait(); err != nil {
		return oci.DescriptorEmptyJSON, err
	}
	if semErr != nil {
		return oci.DescriptorEmptyJSON, fmt.Errorf("failed to acquire lock: %w", semErr)
	}

	if !fetchProgress.CheckAllCompleted() {
		return oci.DescriptorEmptyJSON, fmt.Errorf("failed to fetch all layers")
	}

	config.Created = utils.TimePtr(time.Now())
	config.Architecture = conf.Architecture
	config.OS = conf.OS
	configBytes, err := config.MarshalJSON()
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	artifact := spec.New(pkg, spec.ModelSpec{
		Descriptor: modelspec.ModelDescriptor{
			Name:      conf.ModelFamily,
			CreatedAt: config.Created,
			Family:    conf.ModelFamily,
		},
		Config: modelspec.ModelConfig{
			Format: conf.ModelFormat,
		},
	})

	err = artifact.MarshalYAMLToPath(path)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	artifactBytes, err := artifact.MarshalJSON()
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	configDesc := layerdb.ConfigDesc(configBytes, layerdb.ImageConfigMediaType)
	configDesc.ArtifactType = manifest.Config.MediaType
	configDesc.Data = artifactBytes
	err = o.db.Write(ctx, configDesc, layerdb.BytesToReadCloser(configBytes), nil)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	manifestDesc, manifestBytes, err := layerdb.ManifestDesc(configDesc, oci.Descriptor{}, layers, layerdb.ImageManifestMediaType)
	manifestDesc.ArtifactType = manifest.MediaType
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	err = o.db.Write(ctx, manifestDesc, layerdb.BytesToReadCloser(manifestBytes), nil)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	if reference == "" {
		reference = manifestDesc.Digest.Encoded()
	}

	return manifestDesc, o.db.Index(ctx, reference, manifestDesc)
}

func (o *Ollama) fetchLayer(ctx context.Context,
	remote Repository,
	desc oci.Descriptor,
	diffid digest.Digest,
	ref, path, snapdiff string,
	layers *[]oci.Descriptor,
	layersMu *sync.Mutex,
	config *layerdb.Config,
	pw *progress.ProgressWriter,
	p *progress.Progress, plog *progress.Logger) error {
	if exists, err := o.db.Exists(ctx, desc); err != nil {
		return fmt.Errorf("failed to check local storage: %w", err)
	} else if exists {
		pw.MarkCompleted()
		return nil
	}

	blob, err := remote.Fetch(ctx, desc, diffid, snapdiff, pw)
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	switch desc.MediaType {
	case layerdb.ImageManifestMediaType, layerdb.ImageConfigMediaType, layerdb.ImageTextMediaType:
		if err := o.db.Set(ctx, desc, blob); err != nil {
			return err
		}
	default:
		err := o.output(ctx, desc, diffid, path, blob, layers, layersMu, config, p, plog)
		if err != nil {
			return err
		}
	}

	pw.MarkCompleted()

	return nil
}

func (o *Ollama) output(
	ctx context.Context,
	desc oci.Descriptor,
	diffid digest.Digest,
	dir string,
	blob io.ReadCloser,
	layers *[]oci.Descriptor,
	layersMu *sync.Mutex,
	config *layerdb.Config,
	p *progress.Progress,
	plog *progress.Logger) error {
	filename := desc.Annotations[modelspec.AnnotationFilepath]

	verify := false
	if !isTolerate(desc.MediaType) {
		verify = true
	}

	data, err := o.db.Content(ctx, desc, diffid, blob, verify)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fm := distribution.FileMetadata{}
	fm.Name = filename
	fm.Size = desc.Size
	content := &distribution.Content{
		ID:           desc.Digest.Encoded(),
		Path:         filename,
		MediaType:    layerdb.ImageLayerGzip,
		ArtifactType: desc.MediaType,
		Metadata:     &fm,
		ReadCloser:   io.NopCloser(data),
	}

	pw := progress.NewProgressWriter(plog, tools.ActionBundling, content.Metadata.Name, content.Metadata.Size)
	p.Add(pw)

	return o.dstb.BundleOnce(ctx, content, "", layers, layersMu, config, pw)
}

func (o *Ollama) OverideEndpoint(ctx context.Context, ep string) {}
