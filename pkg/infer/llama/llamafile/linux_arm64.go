//go:build linux && arm64

package llamafile

import _ "embed"

//go:embed assets/llamafile
var llamafileBinary []byte

const LlamafileBinaryName = "llamafile"

func getEmbeddedBinary() ([]byte, string, error) {
	return llamafileBinary, LlamafileBinaryName, nil
}
