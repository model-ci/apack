//go:build !(darwin || linux || windows) || !(amd64 || arm64) || (windows && arm64)

package llamafile

import (
	"fmt"
	"runtime"
)

const LlamafileBinaryName = "llamafile"

func getEmbeddedBinary() ([]byte, string, error) {
	return nil, "", fmt.Errorf("unsupported platform: %s-%s", runtime.GOOS, runtime.GOARCH)
}
