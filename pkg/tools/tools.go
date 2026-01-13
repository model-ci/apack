package tools

import (
	"context"
	"fmt"

	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/model-ci/apack/pkg/progress"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	ActionFetch = "Fetching"
)

type Tool interface {
	Fetch(ctx context.Context, reference string, path string, plog *progress.Logger) (oci.Descriptor, error)
}

type spawn func(user, password string, concurrency int, db layerdb.DB, dstb distribution.Distribution) (Tool, error)

var factory = map[string]spawn{}

func Register(name string, s spawn) {
	if _, ok := factory[name]; !ok {
		factory[name] = s
	}
}

type Tools struct {
	db          layerdb.DB
	dstb        distribution.Distribution
	concurrency int
	set         map[string]Tool
}

func NewTools(db layerdb.DB, dstb distribution.Distribution, concurrency int) (*Tools, error) {
	return &Tools{
		db:          db,
		dstb:        dstb,
		concurrency: concurrency,
		set:         map[string]Tool{},
	}, nil
}

func (t *Tools) Get(user, password, tool string) (Tool, error) {
	id := key(user, password, tool)
	v, ok := t.set[id]
	if !ok {
		fn, ok := factory[tool]
		if !ok {
			return nil, fmt.Errorf("unknown tool: %s", tool)
		}

		tool, err := fn(user, password, t.concurrency, t.db, t.dstb)
		if err != nil {
			return nil, err
		}
		t.set[id] = tool
		return tool, nil
	}
	return v, nil
}

func key(user, password, tool string) string {
	return fmt.Sprintf("%s-%s-%s", user, password, tool)
}
