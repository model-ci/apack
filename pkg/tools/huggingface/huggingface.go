package huggingface

import (
	"context"
	"fmt"
	"net/http"
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

	sem := semaphore.NewWeighted(int64(hf.concurrency))
	fetchProgress := progress.NewProgress()
	errs, errCtx := errgroup.WithContext(ctx)
	fmtErr := func(mediaType string, err error) error {
		if err == nil {
			return nil
		}
		return fmt.Errorf("[tool: %s] failed to fetch %s layer: %w", Name, mediaType, err)
	}

	snapRef, err := hf.dstb.Snapshot(ctx, reference)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	var semErr error
	for _, content := range contents {
		c := content
		if err := c.Overload(ctx, &c); err != nil {
			return oci.DescriptorEmptyJSON, err
		}

		if c.Path == "" {
			log.Logger.Warnf("fetch file path is null: %s", c.MediaType)
			continue
		}

		if err := sem.Acquire(errCtx, 1); err != nil {
			semErr = err
			break
		}

		pw := progress.NewProgressWriter(plog, tools.ActionFetch, c.Metadata.Name, c.Metadata.Size)
		fetchProgress.Add(pw)

		log.Logger.Debugf("[tool: %s] start fetch for: %s, %+v, %+v", Name, c.Path, c, c.Metadata)

		errs.Go(func() error {
			defer sem.Release(1)
			return fmtErr(c.MediaType, hf.dstb.BundleOnce(errCtx, &c, snapRef, &layers, &layersMu, config, pw))
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
