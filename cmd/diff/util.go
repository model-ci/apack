package diff

import (
	"fmt"
	"sort"

	"github.com/model-ci/apack/internal/repo"
	"github.com/model-ci/apack/pkg/progress"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

type DiffResult struct {
	SameConfig       bool
	AnnotationsMatch bool
	SharedLayers     []oci.Descriptor
	UniqueLayersA    []oci.Descriptor
	UniqueLayersB    []oci.Descriptor
}

func CompareManifests(manifestA *oci.Manifest, manifestB *oci.Manifest) *DiffResult {
	result := &DiffResult{}

	result.SameConfig = manifestA.Config.Digest == manifestB.Config.Digest

	numAnnotations := len(manifestA.Annotations)
	if numAnnotations != len(manifestB.Annotations) {
		result.AnnotationsMatch = false
	} else {
		result.AnnotationsMatch = true
		for k, v := range manifestA.Annotations {
			if v2, ok := manifestB.Annotations[k]; !ok || v2 != v {
				result.AnnotationsMatch = false
				break
			}
			numAnnotations--
		}
		if numAnnotations != 0 {
			result.AnnotationsMatch = false
		}
	}

	layerMapA := make(map[string]oci.Descriptor)
	for _, layer := range manifestA.Layers {
		layerMapA[layer.Digest.String()] = layer
	}

	for _, layer := range manifestB.Layers {
		if _, ok := layerMapA[layer.Digest.String()]; ok {
			result.SharedLayers = append(result.SharedLayers, layer)
			delete(layerMapA, layer.Digest.String())
		} else {
			result.UniqueLayersB = append(result.UniqueLayersB, layer)
		}
	}

	result.UniqueLayersA = make([]oci.Descriptor, 0, len(layerMapA))
	for _, layer := range layerMapA {
		result.UniqueLayersA = append(result.UniqueLayersA, layer)
	}

	sort.Slice(result.SharedLayers, func(i, j int) bool {
		return result.SharedLayers[i].MediaType < result.SharedLayers[j].MediaType
	})
	sort.Slice(result.UniqueLayersA, func(i, j int) bool {
		return result.UniqueLayersA[i].MediaType < result.UniqueLayersA[j].MediaType
	})
	sort.Slice(result.UniqueLayersB, func(i, j int) bool {
		return result.UniqueLayersB[i].MediaType < result.UniqueLayersB[j].MediaType
	})

	return result
}

const (
	layerTableHeadings = "Type    | Digest             | Size"
	layerTableFormat   = "%-7s | %-18s | %s\n"
)

func displayLayers(title string, layers []oci.Descriptor) {
	fmt.Println(title)
	fmt.Println("---------------------------------------")
	if len(layers) > 0 {
		fmt.Println(layerTableHeadings)
		for _, layer := range layers {
			fmt.Printf(layerTableFormat,
				repo.FormatMediaType(layer.ArtifactType),
				layer.Digest[:17],
				progress.FormatSize(layer.Size))
		}
	} else {
		fmt.Println("<none>")
	}
	fmt.Println("")
}
