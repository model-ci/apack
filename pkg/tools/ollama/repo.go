package ollama

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/transfer"
	"github.com/model-ci/apack/internal/utils"
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/model-ci/apack/pkg/progress"
	"github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

type Repository interface {
	Resolve(ctx context.Context, tag string) (oci.Descriptor, error)
	Fetch(ctx context.Context, target oci.Descriptor, diffid digest.Digest, snappath string, pw *progress.ProgressWriter) (io.ReadCloser, error)
}

type ollamaRepo struct {
	*remote.Repository
}

func NewRepository(reference string) (*ollamaRepo, error) {
	repo, err := remote.NewRepository(reference)
	if err != nil {
		return nil, err
	}
	return &ollamaRepo{
		Repository: repo,
	}, nil
}

func (o *ollamaRepo) Resolve(ctx context.Context, tag string) (oci.Descriptor, error) {
	desc, _, err := resolve(ctx, o.Repository, tag)
	return desc, err
}

func resolve(ctx context.Context, repo *remote.Repository, tag string) (oci.Descriptor, []byte, error) {
	url := buildRepositoryManifestURL(repo.PlainHTTP, repo.Reference)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return oci.Descriptor{}, nil, err
	}

	manifests := []string{
		layerdb.ImageManifestMediaType,
		layerdb.OCIImageManifestMediaType,
	}

	req.Header.Set("Accept", strings.Join(manifests, ","))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return oci.Descriptor{}, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return oci.Descriptor{}, nil, fmt.Errorf("http %s: %s", resp.Status, string(body))
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return oci.Descriptor{}, nil, err
	}

	mediaType := resp.Header.Get("Content-Type")
	if mediaType == "" || mediaType == "no media type" {
		mediaType = layerdb.ImageManifestMediaType
	}

	desc := oci.Descriptor{
		MediaType: mediaType,
		Digest:    digest.FromBytes(content),
		Size:      int64(len(content)),
	}
	return desc, content, nil
}

func (o *ollamaRepo) Fetch(ctx context.Context, target oci.Descriptor, diffid digest.Digest, snappath string, pw *progress.ProgressWriter) (io.ReadCloser, error) {
	ref := o.Reference
	ref.Reference = target.Digest.String()
	ctx = auth.AppendRepositoryScope(ctx, ref, auth.ActionPull)
	url := buildRepositoryBlobURL(o.PlainHTTP, ref)

	log.Logger.Debugf("Ollama fetching from %s", url)

	finalURL, err := getFinalDownloadURL(ctx, url)
	if err != nil {
		log.Logger.Warnf("Redirect check failed for %s, falling back to original: %v", url, err)
		finalURL = url
	} else {
		log.Logger.Debugf("Ollama redirected to: %s", finalURL)
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
		return nil, nil
	}

	dl := transfer.NewDownloader(finalURL, snapdiff, target.Size, pw)
	if err := dl.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	return os.Open(snapdiff)
}
