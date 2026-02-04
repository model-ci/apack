package infer

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/model-ci/apack/pkg/layerdb"
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
	return &Infer{Config: cfg,
		procs: make(map[string]Proc)}, nil
}

func (i *Infer) Spawning(p *Params) (Proc, oci.Descriptor, error) {
	p.PidFile = filepath.Join(i.Config.PidPath, p.Reference, p.ID)
	p.LogFile = filepath.Join(i.Config.LogPath, p.Reference, p.ID)

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
			layerdb.OCIAnnotationRefName: p.Reference,
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
