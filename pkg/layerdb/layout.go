//go:generate easyjson -all layout.go
package layerdb

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/distribution/distribution/manifest/schema2"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	IndexDir                    = "index"
	RawDir                      = "raw"
	LayerDir                    = "layer"
	Blobs                       = oci.ImageBlobsDir
	Metadata                    = "metadata"
	OCILayoutMediaType          = oci.MediaTypeLayoutHeader
	OCIImageLayoutVersion       = oci.ImageLayoutVersion
	OCIImageLayoutFile          = oci.ImageLayoutFile
	OCIImageIndexFile           = oci.ImageIndexFile
	OCIImageIndexMediaType      = oci.MediaTypeImageIndex
	OCIImageDescriptorMediaType = oci.MediaTypeDescriptor

	OCIImageManifestMediaType = oci.MediaTypeImageManifest
	OCIImageConfigMediaType   = oci.MediaTypeImageConfig
	OCILayer                  = oci.MediaTypeImageLayer
	OCILayerGzip              = oci.MediaTypeImageLayerGzip
	OCILayerZstd              = oci.MediaTypeImageLayerZstd

	ImageManifestMediaType = schema2.MediaTypeManifest
	ImageConfigMediaType   = schema2.MediaTypeImageConfig
	ImageLayer             = schema2.MediaTypeUncompressedLayer
	ImageLayerGzip         = schema2.MediaTypeLayer
	ImageLayerZstd         = OCILayerZstd

	ImageTextMediaType = "text/plain; charset=utf-8"

	OCIImageSnapshotFile = "snapshot.json"

	DockerManifestMediaType  = schema2.MediaTypeManifest
	DockerManifestFile       = "manifest.json"
	DockerConfigFile         = "config.json"
	OCIAnnotationRefName     = oci.AnnotationRefName
	OCIAnnotationCreated     = oci.AnnotationCreated
	OCIAnnotationDeleted     = "org.opencontainers.image.deleted"
	DefaultMediaType         = "application/octet-stream"
	OCIAnnotationSnapshotRef = "org.opencontainers.image.snapshot.ref.name"

	TypeLayers = "layers"
)

var (
	OCIImageIndexVersion  = schema2.SchemaVersion.SchemaVersion
	DockerManifestVersion = OCIImageIndexVersion
)

type Algorithm string

const (
	Gzip        Algorithm = "gzip"
	GzipFastest Algorithm = "gzip-fastest"
	Zstd        Algorithm = "zstd"
	Raw         Algorithm = "raw"
	None        Algorithm = "none"
)

func (a Algorithm) Validate(s string) (Algorithm, error) {
	alg := Algorithm(s)
	switch alg {
	case Gzip, GzipFastest, Zstd, Raw, None:
		return alg, nil
	}
	return "", fmt.Errorf(
		"invalid algorithm %q, must be one of %q, %q, %q, %q, or %q", s, Gzip, GzipFastest, Zstd, Raw, None)
}

type OCILayout struct {
	oci.ImageLayout
	MediaType string `json:"mediaType"`
}

type Index struct {
	oci.Index
}

type Manifest struct {
	oci.Manifest
}

type Descriptor struct {
	oci.Descriptor
}

type Config struct {
	oci.Image
}

func (c *Config) UnmarshalJSONFromIO(rr io.Reader) error {
	b, err := io.ReadAll(rr)
	if err != nil {
		return err
	}

	return c.UnmarshalJSON(b)
}

func blobKey(desc oci.Descriptor) string {
	return filepath.Join(oci.ImageBlobsDir, desc.Digest.Algorithm().String(), desc.Digest.Encoded())
}
