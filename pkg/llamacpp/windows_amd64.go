//go:build windows

package llamacpp

import _ "embed"

//go:embed assets/llama-server-windows-amd64
var llamafileBinary []byte

const LlamafileBinaryName = "llamafile.exe"

func getEmbeddedBinary() ([]byte, string, error) {
	return llamafileBinary, LlamafileBinaryName, nil
}
