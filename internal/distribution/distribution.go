//go:generate easyjson distribution.go
package distribution

import (
	"context"
	"os"

	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/model-ci/apack/pkg/progress"
	modelspec "github.com/modelpack/model-spec/specs-go/v1"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry"
)

type Distribution interface {
	Pull(ctx context.Context, remote registry.Repository, ref registry.Reference, log *progress.Logger, opts *Options) (oci.Descriptor, error)
	Push(ctx context.Context, remote registry.Repository, ref registry.Reference, log *progress.Logger, opts *Options) (oci.Descriptor, error)
	Bundle(ctx context.Context, mf Makefile, log *progress.Logger, opts *Options) (oci.Descriptor, error)
	Extract(ctx context.Context, path string, reference string, log *progress.Logger, opts *Options) error
	Artifacts(ctx context.Context) ([]spec.Artifact, error)
	Artifact(ctx context.Context, reference string) (spec.Artifact, error)
	Manifest(ctx context.Context, reference string) (oci.Manifest, oci.Descriptor, error)
	Config(ctx context.Context, desc oci.Descriptor) (layerdb.Config, error)
	Remove(ctx context.Context, reference string) error
	State(ctx context.Context, reference string, desc oci.Descriptor) error
	Statuses(ctx context.Context) ([]oci.Descriptor, error)
	Snapshot(ctx context.Context, reference string) (string, error)
	Snappath(ctx context.Context, reference string) string
	Tag(ctx context.Context, ref1, ref2 registry.Reference) error
}

type Content struct {
	Path         string
	MediaType    string
	ArtifactType string
	Metadata     *FileMetadata
	Data         *os.File
}

type Makefile interface {
	Reference() string
	Contents() ([]Content, error)
	ConfigMediaType() string
	ArtifactConfigType() string
	ManifestMediaType() string
	ArtifactManifestType() string
	Artifact() spec.Artifact
}

type Snapfile interface {
	Reference() string
	Contents() ([]Content, error)
	ConfigMediaType() string
	ArtifactConfigType() string
	ManifestMediaType() string
	ArtifactManifestType() string
	Artifact() spec.Artifact
}

//easyjson:json
type FileMetadata struct {
	modelspec.FileMetadata
}
