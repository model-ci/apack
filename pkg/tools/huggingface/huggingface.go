package huggingface

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/utils"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/model-ci/apack/pkg/progress"
	"github.com/model-ci/apack/pkg/tools"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	Name = "hf"
)

func init() {
	tools.Register("hf", NewHuggingface)
}

type Huggingface struct {
	username    string
	token       string
	concurrency int
	endpoint    string
	db          layerdb.DB
	dstb        distribution.Distribution
	client      *http.Client
}

func NewHuggingface(user, password string, concurrency int, db layerdb.DB, dstb distribution.Distribution) (tools.Tool, error) {
	client := &http.Client{
		Timeout: 600 * time.Second,
	}
	return &Huggingface{
		username:    user,
		token:       password,
		concurrency: concurrency,
		db:          db,
		dstb:        dstb,
		client:      client,
	}, nil
}

func (hf *Huggingface) OverideEndpoint(_ context.Context, ep string) {
	hf.endpoint = ep
}

func (hf *Huggingface) Fetch(ctx context.Context, reference string, path string, plog *progress.Logger) (oci.Descriptor, error) {
	config := &layerdb.Config{}
	var layersMu sync.Mutex
	var layers []oci.Descriptor

	repo := ctx.Value("repo").(string)
	branch := ctx.Value("branch").(string)

	remote, err := NewRepository(repo, branch, hf.endpoint, hf.token, layerdb.Gzip)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	mf, err := remote.Resolve(ctx, reference)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	contents, err := mf.Contents(ctx)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	log.Logger.Debugf("[tool: %s] start fetch for: %s", Name, reference)

	fetchSem := semaphore.NewWeighted(int64(hf.concurrency))
	fetchProgress := progress.NewProgress()
	fetchErrs, fetchErrCtx := errgroup.WithContext(ctx)
	fmtErr := func(mediaType string, err error) error {
		if err == nil {
			return nil
		}
		return fmt.Errorf("[tool: %s] failed to %s layer: %w", Name, mediaType, err)
	}

	snapDiff, err := hf.dstb.Snapdiff(ctx)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	snapRef, err := hf.dstb.Snapshot(ctx, reference)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	var fetchSemErr error
	for _, content := range contents {
		c := content
		if c.Size() == 0 {
			log.Logger.Warnf("fetch file size is zero: %s", c.Path)
			continue
		}

		if c.Path == "" {
			log.Logger.Warnf("fetch file path is null: %s", c.MediaType)
			continue
		}

		if err := fetchSem.Acquire(fetchErrCtx, 1); err != nil {
			fetchSemErr = err
			break
		}

		pw := progress.NewProgressWriter(plog, tools.ActionFetch, c.Metadata.Name, c.Metadata.Size)
		fetchProgress.Add(pw)

		log.Logger.Debugf("[tool: %s] start fetch for: %s, %+v, %+v", Name, c.Path, c, c.Metadata)

		fetchErrs.Go(func() error {
			defer fetchSem.Release(1)
			return fmtErr(c.MediaType, remote.Fetch(ctx, c, snapDiff, pw))
		})
	}

	if err := fetchErrs.Wait(); err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	if fetchSemErr != nil {
		return oci.DescriptorEmptyJSON, fmt.Errorf("fetch artifact failed to acquire lock: %w", fetchSemErr)
	}

	if !fetchProgress.CheckAllCompleted() {
		return oci.DescriptorEmptyJSON, fmt.Errorf("failed to fetch all layers")
	}

	bundleSem := semaphore.NewWeighted(int64(hf.concurrency))
	bundleProgress := progress.NewProgress()
	bundleErrs, bundleErrCtx := errgroup.WithContext(ctx)

	var bundleSemErr error
	for _, content := range contents {
		c := content
		if c.Size() == 0 {
			continue
		}

		if c.Path == "" {
			log.Logger.Warnf("bundle file path is null: %s", c.MediaType)
			continue
		}

		if err := openfile(filepath.Join(snapDiff, c.ID), &c); err != nil {
			return oci.DescriptorEmptyJSON, err
		}

		if err := bundleSem.Acquire(bundleErrCtx, 1); err != nil {
			bundleSemErr = err
			break
		}

		pw := progress.NewProgressWriter(plog, tools.ActionBundling, c.Metadata.Name, c.Metadata.Size)
		bundleProgress.Add(pw)

		log.Logger.Debugf("[tool: %s] start bundle for: %s, %+v, %+v", Name, c.Path, c, c.Metadata)

		bundleErrs.Go(func() error {
			defer bundleSem.Release(1)
			return fmtErr(c.MediaType, hf.dstb.BundleOnce(bundleErrCtx, &c, "", &layers, &layersMu, config, pw))
		})
	}

	if err := bundleErrs.Wait(); err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	if bundleSemErr != nil {
		return oci.DescriptorEmptyJSON, fmt.Errorf("failed to acquire lock: %w", bundleSemErr)
	}

	if !bundleProgress.CheckAllCompleted() {
		return oci.DescriptorEmptyJSON, fmt.Errorf("failed to bundle all layers")
	}

	config.Created = utils.TimePtr(time.Now())

	configBytes, err := config.MarshalJSON()
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	artifact := mf.Artifact()
	err = artifact.MarshalYAMLToPath(snapRef)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	artifactBytes, err := artifact.MarshalJSON()
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	configDesc := layerdb.ConfigDesc(configBytes, mf.ConfigMediaType())
	configDesc.ArtifactType = mf.ArtifactConfigType()
	configDesc.Data = artifactBytes
	err = hf.db.Write(ctx, configDesc, layerdb.BytesToReadCloser(configBytes), nil)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	manifestDesc, manifestBytes, err := layerdb.ManifestDesc(configDesc, oci.Descriptor{}, layers, mf.ManifestMediaType())
	manifestDesc.ArtifactType = mf.ArtifactManifestType()
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	err = hf.db.Write(ctx, manifestDesc, layerdb.BytesToReadCloser(manifestBytes), nil)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	if reference == "" {
		reference = manifestDesc.Digest.Encoded()
	}

	return manifestDesc, hf.db.Index(ctx, reference, manifestDesc)
}
