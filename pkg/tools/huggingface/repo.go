package huggingface

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/transfer"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/model-ci/apack/pkg/progress"
	"github.com/model-ci/apack/pkg/utils"
)

type Repository interface {
	Resolve(ctx context.Context, reference string) (distribution.Makefile, error)
	Fetch(ctx context.Context, c distribution.Content, snappath string, pw *progress.ProgressWriter) error
	EnableProxy()
}

type huggingFaceRepo struct {
	client   *http.Client
	repo     string
	branch   string
	endpoint string
	token    string
	proxy    bool
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

func (r *huggingFaceRepo) EnableProxy() {
	r.proxy = true
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

func (r *huggingFaceRepo) Fetch(ctx context.Context, c distribution.Content, snapdir string, pw *progress.ProgressWriter) error {
	url := fmt.Sprintf(ResolveURL, r.endpoint, r.repo, r.branch, c.Path)

	log.Logger.Debugf("Fetching from %s", url)

	diffid := filepath.Join(snapdir, c.ID)
	if utils.FileExist(diffid) {
		log.Logger.Warnf("Diff file already exists: %s", diffid)
		pw.MarkCompleted()
		return nil
	}

	var dl *transfer.Downloader
	if r.proxy {
		dl = transfer.NewDownloader(url, diffid, c.Size(), pw, transfer.WithProxy())
	} else {
		dl = transfer.NewDownloader(url, diffid, c.Size(), pw)
	}

	if err := dl.Start(ctx); err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return nil
}
