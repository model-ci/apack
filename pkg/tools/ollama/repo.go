package ollama

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
)

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
	scheme := "https"
	if repo.PlainHTTP {
		scheme = "http"
	}
	url := fmt.Sprintf("%s://%s/v2/%s/manifests/%s",
		scheme, repo.Reference.Registry, repo.Reference.Repository, tag)

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
