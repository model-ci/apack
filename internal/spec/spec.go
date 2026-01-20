//go:generate easyjson -all spec.go
package spec

import (
	"io"
	"os"

	"github.com/goccy/go-yaml"
	modelspec "github.com/modelpack/model-spec/specs-go/v1"
)

const (
	DefaultArtifactName = "Apackfile"
	IgnoreFileName      = ".apackignore"
)

type Artifact struct {
	OCIVersion string    `json:"ociVersion" yaml:"ociVersion"`
	Package    Package   `json:"package,omitempty" yaml:"package,omitempty"`
	Spec       ModelSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

type Package struct {
	Name      string    `json:"name,omitempty" yaml:"name,omitempty"`
	Workspace string    `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Reference string    `json:"reference,omitempty" yaml:"reference,omitempty"`
	Size      int64     `json:"size,omitempty" yaml:"size,omitempty"`
	Digest    string    `json:"digest,omitempty" yaml:"digest,omitempty"`
	Models    []Model   `json:"models,omitempty" yaml:"models,omitempty"`
	DataSets  []DataSet `json:"datasets,omitempty" yaml:"datasets,omitempty"`
	Codes     []Code    `json:"codes,omitempty" yaml:"codes,omitempty"`
	Docs      []Doc     `json:"docs,omitempty" yaml:"docs,omitempty"`
}

type Model struct {
	ID          string `json:"id,omitempty" yaml:"id,omitempty"`
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	Type        string `json:"type,omitempty" yaml:"type,omitempty"`
	Path        string `json:"path,omitempty" yaml:"path,omitempty"`
	Size        int64  `json:"size,omitempty" yaml:"size,omitempty"`
	License     string `json:"license,omitempty" yaml:"license,omitempty"`
	Extensions  any    `json:"extensions,omitempty" yaml:"extensions,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type Code struct {
	ID          string `json:"id,omitempty" yaml:"id,omitempty"`
	Path        string `json:"path,omitempty" yaml:"path,omitempty"`
	Size        int64  `json:"size,omitempty" yaml:"size,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	License     string `json:"license,omitempty" yaml:"license,omitempty"`
}

type DataSet struct {
	ID          string `json:"id,omitempty" yaml:"id,omitempty"`
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	Path        string `json:"path,omitempty" yaml:"path,omitempty"`
	Size        int64  `json:"size,omitempty" yaml:"size,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	License     string `json:"license,omitempty" yaml:"license,omitempty"`
	Extensions  any    `json:"extensions,omitempty" yaml:"extensions,omitempty"`
}

type Doc struct {
	ID          string `json:"id,omitempty" yaml:"id,omitempty"`
	Path        string `json:"path" yaml:"path"`
	Size        int64  `json:"size" yaml:"size"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type ModelSpec struct {
	Descriptor modelspec.ModelDescriptor `json:"descriptor"`
	Config     modelspec.ModelConfig     `json:"config,omitempty"`
}

func New(p Package, s ModelSpec) *Artifact {
	return &Artifact{
		OCIVersion: OCIVersion,
		Package:    p,
		Spec:       s,
	}
}

func NewModelSpec() ModelSpec {
	return ModelSpec{}
}

func (a *Artifact) MarshalToYAML() ([]byte, error) {
	return yaml.Marshal(a)
}

func (a *Artifact) UnmarshalYamlFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, a)
}

func (a *Artifact) UnmarshalJSONFromIO(rr io.Reader) error {
	b, err := io.ReadAll(rr)
	if err != nil {
		return err
	}

	return a.UnmarshalJSON(b)
}
