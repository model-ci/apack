//go:build darwin && arm64

package llamacpp

import _ "embed"

//go:embed assets/llama-server-darwin-arm64
var llamafileBinary []byte

const LlamafileBinaryName = "llama-server"

func getEmbeddedBinary() ([]byte, string, error) {
	return llamafileBinary, LlamafileBinaryName, nil
}
