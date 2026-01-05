package distribution

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/internal/utils"
	"github.com/model-ci/apack/pkg/layerdb"
	modelspec "github.com/modelpack/model-spec/specs-go/v1"
)

type makefile struct {
	artifact   spec.Artifact
	reference  string
	algo       layerdb.Algorithm
	compatible bool
}

func NewMakefile(artifact spec.Artifact, reference string, algo layerdb.Algorithm, compatible bool) Makefile {
	return &makefile{
		artifact:   artifact,
		reference:  reference,
		algo:       algo,
		compatible: compatible,
	}
}

func (m *makefile) Reference() string {
	return m.reference
}

func (m *makefile) Contents() ([]Content, error) {
	var contents []Content
	for _, model := range m.artifact.Package.Models {
		mediaType := spec.OCIDetermineMediaType(m.algo)
		if m.compatible {
			mediaType = spec.CompatibleOCIDetermineMediaType(m.algo)
		}
		artifactType := spec.InferMediaType(model.Path, m.algo)
		c, err := m.buildContent(mediaType, artifactType, model.Path)
		if err != nil {
			return nil, err
		}
		contents = append(contents, *c)
	}

	for _, datasets := range m.artifact.Package.DataSets {
		mediaType := spec.OCIDetermineMediaType(m.algo)
		if m.compatible {
			mediaType = spec.CompatibleOCIDetermineMediaType(m.algo)
		}
		artifactType := spec.DetermineMediaType(spec.FileTypeDataset, m.algo)
		c, err := m.buildContent(mediaType, artifactType, datasets.Path)
		if err != nil {
			return nil, err
		}
		contents = append(contents, *c)
	}

	/*
		for _, code := range m.artifact.Codes {
			mediaType := spec.OCIDetermineMediaType(m.algo)
			artifactType := spec.DetermineMediaType(spec.FileTypeDataset, m.algo)
			c, err := m.buildContent(mediaType, artifactType, code.Path)
			if err != nil {
				return nil, err
			}
			contents = append(contents, *c)
		}
	*/

	for _, doc := range m.artifact.Package.Docs {
		mediaType := spec.OCIDetermineMediaType(m.algo)
		if m.compatible {
			mediaType = spec.CompatibleOCIDetermineMediaType(m.algo)
		}
		artifactType := spec.DetermineMediaType(spec.FileTypeDocs, m.algo)
		c, err := m.buildContent(mediaType, artifactType, doc.Path)
		if err != nil {
			return nil, err
		}
		contents = append(contents, *c)
	}

	return contents, nil
}

func (m *makefile) buildContent(mediaType, artifactType string, filename string) (*Content, error) {
	c := &Content{
		Path:         filename,
		MediaType:    mediaType,
		ArtifactType: artifactType,
		Metadata:     &FileMetadata{},
	}

	path := filepath.Join(m.artifact.Package.Workspace, filename)
	if !utils.FileExist(path) {
		return nil, fmt.Errorf("file %s does not exist", c.Path)
	}

	var err error
	c.Data, err = os.Open(path)
	if err != nil {
		return nil, err
	}

	fi, err := c.Data.Stat()
	if err != nil {
		return nil, err
	}

	return c, c.Metadata.Fill(fi)
}

func (m *makefile) ConfigMediaType() string {
	if m.compatible {
		return layerdb.ImageConfigMediaType
	}
	return layerdb.OCIImageConfigMediaType
}

func (m *makefile) ManifestMediaType() string {
	if m.compatible {
		return layerdb.ImageManifestMediaType
	}
	return layerdb.OCIImageManifestMediaType
}

func (m *makefile) ArtifactConfigType() string {
	return modelspec.MediaTypeModelConfig
}

func (m *makefile) ArtifactManifestType() string {
	return modelspec.ArtifactTypeModelManifest
}

func (m *makefile) SubjectMediaType() string {
	return "application/vnd.apack.artifact.config.v1+json"
}

func (m *makefile) Artifact() spec.Artifact {
	return m.artifact
}
