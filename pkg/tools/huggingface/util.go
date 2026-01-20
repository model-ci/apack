package huggingface

import (
	"os"
	"strings"

	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/pkg/distribution"
)

const (
	Endpoint   = "huggingface.co"
	ResolveURL = "https://%s/%s/resolve/%s/%s"
	TreeURL    = "https://%s/api/models/%s/tree/%s"
)

func makeArtifact(dir *spec.Directory, repo string) (*spec.Artifact, error) {
	sections := strings.Split(repo, "/")
	model := spec.ModelSpec{}
	if len(sections) >= 2 {
		model.Descriptor.Name = sections[len(sections)-1]
		model.Descriptor.Authors = []string{sections[len(sections)-2]}
	}

	return spec.Gen(dir, &spec.Artifact{Spec: model})
}

func openfile(path string, c *distribution.Content) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	c.ReadCloser = f

	fi, err := f.Stat()
	if err != nil {
		return err
	}
	
	return c.Metadata.Fill(fi)
}
