package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"
)

const (
	Registry  = "registry.ollama.ai"
	Namespace = "library"
)

type RootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

type OllamaConfig struct {
	ModelFormat   string   `json:"model_format"`
	ModelFamily   string   `json:"model_family"`
	ModelFamilies []string `json:"model_families"`
	ModelType     string   `json:"model_type"`
	FileType      string   `json:"file_type"`
	Architecture  string   `json:"architecture"`
	OS            string   `json:"os"`
	RootFS        RootFS   `json:"rootfs"`
}

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

func getConfig(ctx context.Context, store oras.ReadOnlyTarget, configDesc oci.Descriptor) (*OllamaConfig, error) {
	configBytes, err := content.FetchAll(ctx, store, configDesc)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", configDesc.Digest, err)
	}

	config := OllamaConfig{}
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", configDesc.Digest, err)
	}

	return &config, nil
}

func getFilename(media string, id string, size int64, oc *OllamaConfig, pkg *spec.Package) string {
	switch media {
	case "application/vnd.ollama.image.model":
		name := fmt.Sprintf("%s.%s", oc.FileType, oc.ModelFormat)
		pkg.Models = append(pkg.Models, spec.Model{ID: id, Path: name, Size: size})
		return name
	case "application/vnd.ollama.image.prompt", "application/vnd.ollama.image.template":
		name := "TEMPLATE"
		pkg.Codes = append(pkg.Codes, spec.Code{ID: id, Path: name, Size: size})
		return name
	case "application/vnd.ollama.image.system":
		name := "SYSTEM"
		pkg.Codes = append(pkg.Codes, spec.Code{ID: id, Path: name, Size: size})
		return name
	case "application/vnd.ollama.image.params":
		name := "params"
		pkg.Docs = append(pkg.Docs, spec.Doc{ID: id, Path: name, Size: size})
		return name
	case "application/vnd.ollama.image.messages":
		return "messages"
	case "application/vnd.ollama.image.license":
		name := "LICENSE"
		pkg.Docs = append(pkg.Docs, spec.Doc{ID: id, Path: name, Size: size})
		return name
	}
	return "unknown"
}

func checkFileMode(r io.Reader) os.FileMode {
	f, ok := r.(*os.File)
	if !ok {
		return 0o666
	}

	info, err := f.Stat()
	if err != nil {
		return 0o666
	}

	return info.Mode()
}

func MakeReference(ref string) registry.Reference {
	ref = strings.TrimSpace(ref)

	var path, reference string

	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		path = ref[:idx]
		reference = ref[idx+1:]
	} else if idx := strings.LastIndex(ref, ":"); idx != -1 {
		path = ref[:idx]
		reference = ref[idx+1:]
	} else {
		path = ref
		reference = "latest"
	}

	var fullRepository string
	if strings.Contains(path, "/") {
		fullRepository = path
	} else {
		fullRepository = Namespace + "/" + path
	}

	return registry.Reference{
		Registry:   Registry,
		Repository: fullRepository,
		Reference:  reference,
	}
}

func isTolerate(media string) bool {
	return media == "application/vnd.ollama.image.template"
}

func makeContent() *distribution.Content {
	return &distribution.Content{}
}
