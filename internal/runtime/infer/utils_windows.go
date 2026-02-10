//go:build windows
// +build windows

package infer

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/model-ci/apack/internal/types"
)

// GetDefaultExecutableName returns the executable name for Windows.
func GetDefaultExecutableName() string {
	path := os.Getenv(types.ENV_LLAMACPP_BINARY_PATH)
	if path != "" {
		return path
	}
	return "llama-server.exe"
}

// isProcessAlive checks if the process is running using OpenProcess on Windows.
func isProcessAlive(pid int) bool {
	const PROCESS_QUERY_INFORMATION = 0x0400

	// Try to open the process handle
	h, err := syscall.OpenProcess(PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		// Failed to open process, usually means it doesn't exist or access denied
		// For stricter check, one might check specific error codes,
		// but generally open fail = not running for our child process case.
		return false
	}

	// If successful, close handle and return true
	syscall.CloseHandle(h)
	return true
}

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
}

func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		// CREATE_NEW_PROCESS_GROUP = 0x0200
		// DETACHED_PROCESS = 0x00000008
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
