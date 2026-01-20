package huggingface

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/utils"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/model-ci/apack/pkg/progress"
)

type Repository interface {
	Resolve(ctx context.Context, reference string) (distribution.Makefile, error)
	Fetch(ctx context.Context, c distribution.Content, snappath string, pw *progress.ProgressWriter) error
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

func (r *huggingFaceRepo) Fetch(ctx context.Context, c distribution.Content, snapdir string, pw *progress.ProgressWriter) error {
	url := fmt.Sprintf(ResolveURL, r.endpoint, r.repo, r.branch, c.Path)

	log.Logger.Debugf("Fetching from %s", url)

	tmpfile := filepath.Join(snapdir, c.ID)
	realfile := filepath.Join(snapdir, c.Path)

	if utils.FileExist(realfile) {
		log.Logger.Warnf("File already exists: %s", realfile)
		pw.MarkCompleted()
		return nil
	}

	if utils.FileExist(tmpfile) {
		log.Logger.Warnf("File already exists: %s, need to rename it to %s", tmpfile, realfile)
		pw.MarkCompleted()
		return os.Rename(tmpfile, realfile)
	}

	dl := NewDownloader(url, tmpfile, c.Size(), pw)
	if err := dl.Start(ctx); err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return os.Rename(tmpfile, realfile)
}
