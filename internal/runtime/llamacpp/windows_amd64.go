//go:build windows

package llamacpp

import _ "embed"

//go:embed assets/llamafile.exe
var llamafileBinary []byte

const LlamafileBinaryName = "llamafile.exe"

func getEmbeddedBinary() ([]byte, string, error) {
	return llamafileBinary, LlamafileBinaryName, nil
}
