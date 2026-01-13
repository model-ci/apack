package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/model-ci/apack/pkg/layerdb"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"

	modelspec "github.com/modelpack/model-spec/specs-go/v1"
)

const (
	DefaultRegistry   = "localhost"
	DefaultRepository = "_"
)

var (
	startEndAlphanumeric = regexp.MustCompile(`[a-z0-9](.*[a-z0-9])?`)
)

func DefaultReference() *registry.Reference {
	return &registry.Reference{
		Registry:   DefaultRegistry,
		Repository: DefaultRepository,
	}
}

func FormatRepositoryForDisplay(repo string) string {
	repo = strings.TrimPrefix(repo, DefaultRegistry+"/")
	repo = strings.TrimPrefix(repo, DefaultRepository)
	repo = strings.TrimPrefix(repo, "@")
	return repo
}

func RepoPath(storagePath string, ref *registry.Reference) string {
	return filepath.Join(storagePath, ref.Registry, ref.Repository)
}

func GetManifest(ctx context.Context, store oras.ReadOnlyTarget, manifestDesc oci.Descriptor) (*layerdb.Manifest, error) {
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

func GetConfig(ctx context.Context, store oras.ReadOnlyTarget, configDesc oci.Descriptor) (*layerdb.Config, error) {
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

func ResolveManifest(ctx context.Context, store oras.Target, reference string) (oci.Descriptor, *layerdb.Manifest, error) {
	desc, err := store.Resolve(ctx, reference)
	if err != nil {
		return oci.DescriptorEmptyJSON, nil, fmt.Errorf("reference %s not found in repository: %w", reference, err)
	}
	manifest, err := GetManifest(ctx, store, desc)
	if err != nil {
		return oci.DescriptorEmptyJSON, nil, err
	}
	return desc, manifest, nil
}

func ParseImageReference(ref string) (reg, repo, tag string, err error) {
	parsedRef, err := registry.ParseReference(ref)
	if err != nil {
		return "", "", "", err
	}

	reg = parsedRef.Registry
	repo = parsedRef.Repository
	tag = parsedRef.Reference

	if tag == "" {
		tag = "latest"
	}

	return reg, repo, tag, nil
}

func FormatMediaType(s string) string {
	switch s {
	case modelspec.MediaTypeModelWeight, modelspec.MediaTypeModelWeightRaw, modelspec.MediaTypeModelWeightGzip, modelspec.MediaTypeModelWeightZstd:
		return "weight"
	case modelspec.MediaTypeModelCode:
		return "code"
	case modelspec.MediaTypeModelDoc:
		return "doc"
	case modelspec.MediaTypeModelWeightConfig:
		return "config"

	default:
		return "unknown"
	}
}

var (
	ErrNotFountLinkFile = errors.New("not found link file")
)

func WriteLinkFile(dirPath string, data string) error {
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	targetPath := filepath.Join(dirPath, ".link")

	if err := os.WriteFile(targetPath, []byte(data), 0644); err != nil {
		return fmt.Errorf("failed to write .link file: %w", err)
	}

	return nil
}

func ReadLinkFile(dirPath string) (string, error) {
	targetPath := filepath.Join(dirPath, ".link")

	content, err := os.ReadFile(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

func IsLinkFileExist(dirPath string) bool {
	targetPath := filepath.Join(dirPath, ".link")
	_, err := os.Stat(targetPath)
	// If err is nil, file exists. If err is "not exist", file doesn't exist.
	return !os.IsNotExist(err)
}