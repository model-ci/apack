package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/internal/utils"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
	modelspec "github.com/modelpack/model-spec/specs-go/v1"
	"github.com/opencontainers/go-digest"
)

type makefile struct {
	artifact   spec.Artifact
	reference  string
	algo       layerdb.Algorithm
	compatible bool
}

func NewMakefile(artifact spec.Artifact, reference string, algo layerdb.Algorithm, compatible bool) distribution.Makefile {
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

func (m *makefile) Contents(_ context.Context) ([]distribution.Content, error) {
	var contents []distribution.Content
	for i, model := range m.artifact.Package.Models {
		mediaType := spec.OCIDetermineMediaType(m.algo)
		if m.compatible {
			mediaType = spec.CompatibleOCIDetermineMediaType(m.algo)
		}
		artifactType := spec.InferMediaType(model.Path, m.algo)
		c, err := m.buildContent(mediaType, artifactType, model.Path)
		if err != nil {
			return nil, err
		}
		m.artifact.Package.Models[i].ID = c.ID
		m.artifact.Package.Models[i].Path = c.Path	
		m.artifact.Package.Models[i].Size = c.Size()
		contents = append(contents, *c)
	}

	for i, datasets := range m.artifact.Package.DataSets {
		mediaType := spec.OCIDetermineMediaType(m.algo)
		if m.compatible {
			mediaType = spec.CompatibleOCIDetermineMediaType(m.algo)
		}
		artifactType := spec.DetermineMediaType(spec.FileTypeDataset, m.algo)
		c, err := m.buildContent(mediaType, artifactType, datasets.Path)
		if err != nil {
			return nil, err
		}
		m.artifact.Package.DataSets[i].ID = c.ID
		m.artifact.Package.DataSets[i].Path = c.Path	
		m.artifact.Package.DataSets[i].Size = c.Size()
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

	for i, doc := range m.artifact.Package.Docs {
		mediaType := spec.OCIDetermineMediaType(m.algo)
		if m.compatible {
			mediaType = spec.CompatibleOCIDetermineMediaType(m.algo)
		}
		artifactType := spec.DetermineMediaType(spec.FileTypeDocs, m.algo)
		c, err := m.buildContent(mediaType, artifactType, doc.Path)
		if err != nil {
			return nil, err
		}
		m.artifact.Package.Docs[i].ID = c.ID
		m.artifact.Package.Docs[i].Path = c.Path	
		m.artifact.Package.Docs[i].Size = c.Size()
		contents = append(contents, *c)
	}

	return contents, nil
}

func (m *makefile) buildContent(mediaType, artifactType string, filename string) (*distribution.Content, error) {
	c := &distribution.Content{
		Path:         filename,
		MediaType:    mediaType,
		ArtifactType: artifactType,
		Metadata:     &distribution.FileMetadata{},
	}

	path := filepath.Join(m.artifact.Package.Workspace, filename)
	if !utils.FileExist(path) {
		return nil, fmt.Errorf("file %s does not exist", c.Path)
	}

	var err error
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	dgt, err := digest.FromReader(f)
	if err != nil {
		return nil, err
	}
	c.ID = dgt.Encoded()

	if _, err := f.Seek(0, 0); err != nil {
        return nil, err
    }
	c.ReadCloser = f

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	log.Logger.Debugf("buildContent: name=%s size=%d digest=%s", fi.Name(), fi.Size(), c.ID)

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
