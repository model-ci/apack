//go:build linux && amd64

package llamacpp

import _ "embed"

//go:embed assets/llama-server-linux-amd64
var llamafileBinary []byte

const LlamafileBinaryName = "llama-server"

func getEmbeddedBinary() ([]byte, string, error) {
	return llamafileBinary, LlamafileBinaryName, nil
}
