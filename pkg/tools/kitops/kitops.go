package kitops

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/model-ci/apack/pkg/progress"
	"github.com/model-ci/apack/pkg/tools"
	"github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

func init() {
	tools.Register("kitops", NewKitops)
}

type Kitops struct {
	username    string
	password    string
	concurrency int
	db          layerdb.DB
	dstb        distribution.Distribution
}

func NewKitops(user, password string, concurrency int, db layerdb.DB, dstb distribution.Distribution) (tools.Tool, error) {
	return &Kitops{
		username:    user,
		password:    password,
		concurrency: concurrency,
		db:          db,
		dstb:        dstb,
	}, nil
}

func (k *Kitops) Get(ctx context.Context, serverAddress string) (auth.Credential, error) {
	return auth.Credential{
		Username: k.username,
		Password: k.password,
	}, nil
}

func (k *Kitops) Fetch(ctx context.Context, reference string, path string, plog *progress.Logger) (oci.Descriptor, error) {
	repo, err := remote.NewRepository(reference)
	if err != nil {
		return oci.DescriptorEmptyJSON, fmt.Errorf("invalid reference %q: %w", reference, err)
	}

	if k.username != "" || k.password != "" {
		repo.Client = &auth.Client{
			Client:     retry.DefaultClient,
			Cache:      auth.NewCache(),
			Credential: k.Get,
		}
	}

	desc, err := repo.Resolve(ctx, repo.Reference.Reference)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	manifest, err := getManifest(ctx, repo, desc)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	config, err := getConfig(ctx, repo, manifest.Config)
	if err != nil {
		return oci.DescriptorEmptyJSON, err
	}

	toPull := manifest.Layers
	toPull = append(toPull, manifest.Config, desc)
	sem := semaphore.NewWeighted(int64(k.concurrency))
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

		pw := progress.NewProgressWriter(plog, tools.ActionFetch, pullDesc.Digest.Encoded()[:12], pullDesc.Size)
		pullProgress.Add(pw)

		errs.Go(func() error {
			defer sem.Release(1)
			diffid := digest.Digest("")
			if pullDescIndex < layers {
				diffid = config.RootFS.DiffIDs[pullDescIndex]
			}
			return fmtErr(pullDesc, k.pullLayer(errCtx, repo, pullDesc, diffid, reference, path, pw))
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

	return desc, k.db.Index(ctx, reference, desc)
}

func (k *Kitops) pullLayer(ctx context.Context, remote registry.Repository, desc oci.Descriptor, diffid digest.Digest, ref, path string, pw *progress.ProgressWriter) error {
	if exists, err := k.db.Exists(ctx, desc); err != nil {
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
		err := k.extract(ctx, desc, diffid, path, blob, pw)
		if err != nil {
			return err
		}
	} else {
		if err := k.db.Set(ctx, desc, blob); err != nil {
			return err
		}
	}

	pw.MarkCompleted()

	return nil
}

func (k *Kitops) extract(ctx context.Context, desc oci.Descriptor, diffid digest.Digest, dir string, content io.ReadCloser, pw *progress.ProgressWriter) error {
	path := desc.Annotations[""]
	cont, err := k.db.Content(ctx, desc, diffid, content, true)
	if err != nil {
		return err
	}
	return k.extractLayer(ctx, filepath.Join(dir, path), cont, pw)
}

func (k *Kitops) extractLayer(ctx context.Context, path string, src io.Reader, pw *progress.ProgressWriter) error {
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
