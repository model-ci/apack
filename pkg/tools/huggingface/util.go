package huggingface

import (
	"context"
	"fmt"
	"strings"

	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/pkg/layerdb"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
)

const (
	Endpoint   = "huggingface.co"
	ResolveURL = "https://%s/%s/resolve/%s/%s"
	TreeURL    = "https://%s/api/models/%s/tree/%s"
)

func getManifest(ctx context.Context, store oras.ReadOnlyTarget, manifestDesc oci.Descriptor) (*layerdb.Manifest, error) {
	manifestBytes, err := content.FetchAll(ctx, store, manifestDesc)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest %s: %w", manifestDesc.Digest, err)
	}

	manifest := &layerdb.Manifest{}
	manifest.UnmarshalJSON(manifestBytes)
	if err := manifest.UnmarshalJSON(manifestBytes); err != nil {
		return nil, fmt.Errorf("failed to parse manifest %s: %w", manifestDesc.Digest, err)
	}

	switch manifest.MediaType {
	case layerdb.ImageManifestMediaType, layerdb.OCIImageManifestMediaType:
		// Supported manifest media types
	default:
		return nil, fmt.Errorf("unsupported manifest media type %s", manifest.MediaType)
	}

	switch manifest.Config.MediaType {
	case layerdb.ImageConfigMediaType, layerdb.OCIImageConfigMediaType:
		// Supported config media types
	default:
		return nil, fmt.Errorf("unsupported config media type %s", manifest.Config.MediaType)
	}

	return manifest, nil
}

func getConfig(ctx context.Context, store oras.ReadOnlyTarget, configDesc oci.Descriptor) (*layerdb.Config, error) {
	configBytes, err := content.FetchAll(ctx, store, configDesc)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", configDesc.Digest, err)
	}

	config := &layerdb.Config{}
	config.UnmarshalJSON(configBytes)
	if err := config.UnmarshalJSON(configBytes); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", configDesc.Digest, err)
	}

	return config, nil
}

func makeArtifact(dir *spec.Directory, repo string) (*spec.Artifact, error) {
	sections := strings.Split(repo, "/")
	model := spec.ModelSpec{}
	if len(sections) >= 2 {
		model.Descriptor.Name = sections[len(sections)-1]
		model.Descriptor.Authors = []string{sections[len(sections)-2]}
	}

	return spec.Gen(dir, &spec.Artifact{Spec: model})
}
