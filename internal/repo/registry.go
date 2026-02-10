package repo

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/transfer"
	"github.com/model-ci/apack/pkg/progress"
	"github.com/model-ci/apack/pkg/utils"
	"github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type Registry interface {
	Resolve(ctx context.Context, reference string) (oci.Descriptor, error)
	Fetch(ctx context.Context, target oci.Descriptor, diffid digest.Digest, snappath string, pw *progress.ProgressWriter) (io.ReadCloser, error)
}

type registryRepo struct {
	*remote.Repository
}

func NewRepository(repo *remote.Repository) *registryRepo {
	return &registryRepo{
		Repository: repo,
	}
}

func (rr *registryRepo) Resolve(ctx context.Context, reference string) (oci.Descriptor, error) {
	return rr.Repository.Resolve(ctx, reference)
}

func (rr *registryRepo) Fetch(ctx context.Context, target oci.Descriptor, diffid digest.Digest, snappath string, pw *progress.ProgressWriter) (io.ReadCloser, error) {
	ref := rr.Reference
	ref.Reference = target.Digest.String()
	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull)
	url := utils.BuildRepositoryBlobURL(rr.PlainHTTP, ref)

	log.Logger.Debugf("Registry fetching from %s", url)

	finalURL, err := utils.GetFinalDownloadURL(ctx, url)
	if err != nil {
		log.Logger.Warnf("Redirect check failed for %s, falling back to original: %v", url, err)
		finalURL = url
	} else {
		log.Logger.Debugf("Registry redirected to: %s", finalURL)
	}

	var snapdiff string

	if diffid.String() != "" {
		snapdiff = filepath.Join(snappath, diffid.Encoded())
	} else {
		snapdiff = filepath.Join(snappath, target.Digest.Encoded())
	}

	if utils.FileExist(snapdiff) {
		log.Logger.Warnf("Diff file already exists: %s", snapdiff)
		pw.MarkCompleted()
		return os.Open(snapdiff)
	}

	dl := transfer.NewDownloader(finalURL, snapdiff, target.Size, pw)
	if err := dl.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	return os.Open(snapdiff)
}