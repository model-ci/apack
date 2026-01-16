package huggingface

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
)

type Repository interface {
	Resolve(ctx context.Context, reference string) (distribution.Makefile, error)
	Fetch(ctx context.Context, c *distribution.Content) error
}

type huggingFaceRepo struct {
	client   *http.Client
	repo     string
	branch   string
	endpoint string
	token    string
	argo     layerdb.Algorithm
}

func NewRepository(repo, branch, endpoint, token string, argo layerdb.Algorithm) (Repository, error) {
	return &huggingFaceRepo{
		client: &http.Client{
			Timeout: 600 * time.Second,
		},
		repo:     repo,
		branch:   branch,
		endpoint: endpoint,
		token:    token,
		argo:     argo,
	}, nil
}

func (r *huggingFaceRepo) Resolve(ctx context.Context, reference string) (distribution.Makefile, error) {
	baseURL, err := url.Parse(fmt.Sprintf(TreeURL, r.endpoint, r.repo, r.branch))
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	dir, err := walkRepoTree(ctx, r.client, r.token, baseURL, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to walk repository tree: %w", err)
	}

	artifact, err := makeArtifact(dir, reference)
	if err != nil {
		return nil, fmt.Errorf("failed to make artifact: %w", err)
	}

	return NewMakefile(*artifact, reference, r.argo, r, true), nil
}

func (r *huggingFaceRepo) Fetch(ctx context.Context, c *distribution.Content) error {
	url := fmt.Sprintf(ResolveURL, r.endpoint, r.repo, r.branch, c.Path)
	log.Logger.Debugf("Fetching from %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to resolve URL: %w", err)
	}
	if r.token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.token))
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("error calling API: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received status code %d when downloading file %s from %s", resp.StatusCode, c.Path, url)
	}

	c.ReadCloser = resp.Body
	return nil
}
