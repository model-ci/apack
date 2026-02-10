//go:build !windows
// +build !windows

package infer

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/model-ci/apack/internal/types"
)

// GetDefaultExecutableName returns the executable name for Unix-like systems.
func GetDefaultExecutableName() string {
	path := os.Getenv(types.ENV_LLAMACPP_BINARY_PATH)
	if path != "" {
		return path
	}
	return "llama-server"
}

// isProcessAlive checks if the process is running using signal 0 on Unix.
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send Signal(0) to probe
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}

	if errors.Is(err, os.ErrProcessDone) {
		return false
	}

	// Check for "no such process" error (ESRCH)
	// Go doesn't expose ESRCH directly in os package, so checking error string is a common fallback
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "process not found") || strings.Contains(errStr, "no such process") {
		return false
	}

	// EPERM means process exists but we don't have permission, so it is alive.
	return true
}

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
}

func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid: true,
	}
}
