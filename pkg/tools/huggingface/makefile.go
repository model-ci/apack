package huggingface

import (
	"context"

	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
	modelspec "github.com/modelpack/model-spec/specs-go/v1"
)

type makefile struct {
	Repository

	artifact   spec.Artifact
	reference  string
	algo       layerdb.Algorithm
	token      string
	compatible bool
}

func NewMakefile(artifact spec.Artifact, reference string, algo layerdb.Algorithm, repo Repository, compatible bool) distribution.Makefile {
	return &makefile{
		Repository: repo,
		artifact:   artifact,
		reference:  reference,
		algo:       algo,
		compatible: compatible,
	}
}

func (m *makefile) Reference() string {
	return m.reference
}

func (m *makefile) Contents(ctx context.Context) ([]distribution.Content, error) {
	var contents []distribution.Content
	for _, model := range m.artifact.Package.Models {
		mediaType := spec.OCIDetermineMediaType(m.algo)
		if m.compatible {
			mediaType = spec.CompatibleOCIDetermineMediaType(m.algo)
		}
		artifactType := spec.InferMediaType(model.Path, m.algo)
		c, err := m.buildContent(ctx, mediaType, artifactType, model.ID, model.Path, model.Size)
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
		c, err := m.buildContent(ctx, mediaType, artifactType, datasets.ID, datasets.Path, datasets.Size)
		if err != nil {
			return nil, err
		}
		contents = append(contents, *c)
	}
	/*
		for _, code := range m.artifact.Package.Codes {
			mediaType := spec.OCIDetermineMediaType(m.algo)
			if m.compatible {
				mediaType = spec.CompatibleOCIDetermineMediaType(m.algo)
			}
			artifactType := spec.DetermineMediaType(spec.FileTypeDataset, m.algo)
			c, err := m.buildContent(ctx, mediaType, artifactType, code.Path, code.Size)
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
		c, err := m.buildContent(ctx, mediaType, artifactType, doc.ID, doc.Path, doc.Size)
		if err != nil {
			return nil, err
		}
		contents = append(contents, *c)
	}

	return contents, nil
}

func (m *makefile) buildContent(ctx context.Context, mediaType, artifactType string, id, filename string, size int64) (*distribution.Content, error) {
	metadata := &distribution.FileMetadata{}
	metadata.Name = filename
	metadata.Size = size
	return &distribution.Content{
		ID:           id,
		Path:         filename,
		MediaType:    mediaType,
		ArtifactType: artifactType,
		Metadata:     metadata,
	}, nil
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
