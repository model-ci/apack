package infer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/model-ci/apack/pkg/llamacpp"
	"github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

type Proc interface {
	Exec(context.Context) error
	Kill(context.Context) error
	IsRunning() bool
}

type Infer struct {
	Config *Config
	procs  map[string]Proc
}

func New(cfg *Config) (*Infer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	binpath := filepath.Join(cfg.BasePath, "bin")
	if err := os.MkdirAll(binpath, 0o755); err != nil {
		return nil, err
	}

	manager, err := llamacpp.NewManager(binpath)
	if err != nil {
		return nil, err
	}

	binary, err := manager.GenBinaryPath()
	if err != nil {
		return nil, err
	}

	if err := manager.SetBinaryPath(binary); err != nil {
		return nil, err
	}

	return &Infer{Config: cfg,
		procs: make(map[string]Proc)}, nil
}

func (i *Infer) Spawning(p *types.Params) (Proc, oci.Descriptor, error) {
	p.PidFile = filepath.Join(i.Config.PidPath, p.Refer, p.ID)
	p.LogFile = filepath.Join(i.Config.LogPath, p.Refer, p.ID)

	proc, err := NewLlamaCppInfer(p)
	if err != nil {
		return nil, oci.DescriptorEmptyJSON, err
	}

	paramBytes, err := p.MarshalJSON()
	if err != nil {
		return nil, oci.DescriptorEmptyJSON, err
	}

	dgt := digest.FromBytes(paramBytes)
	i.procs[p.ID[:12]] = proc

	return proc, oci.Descriptor{
		MediaType: oci.MediaTypeDescriptor,
		Digest:    dgt,
		Size:      int64(len(paramBytes)),
		Data:      paramBytes,
		Annotations: map[string]string{
			layerdb.OCIAnnotationRefName: p.Refer,
		},
	}, nil
}

func (i *Infer) Status(id string) (Proc, bool) {
	v, ok := i.procs[id]
	return v, ok
}

func (i *Infer) Stop(id string) error {
	if v, ok := i.procs[id]; ok {
		return v.Kill(context.Background())
	}
	return fmt.Errorf("process not found: %s", id)
}
