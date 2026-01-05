//go:build windows

package llamafile

import _ "embed"

//go:embed assets/llamafile
var llamafileBinary []byte

const LlamafileBinaryName = "llamafile.exe"

func getEmbeddedBinary() ([]byte, string, error) {
	return llamafileBinary, LlamafileBinaryName, nil
}
