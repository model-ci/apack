package infer

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// isRunning checks if the service specified by the PID file is currently running.
func isRunning(pidFilePath string) bool {
	// 1. Read PID file
	content, err := os.ReadFile(pidFilePath)
	if err != nil {
		return false // File does not exist, assume not running
	}

	// 2. Parse PID
	pidStr := strings.TrimSpace(string(content))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false
	}

	// 3. Check if process is alive
	return isProcessAlive(pid)
}

// killFromPidFile terminates the process identified by the PID file.
func killFromPidFile(pidFilePath string) error {
	if !isRunning(pidFilePath) {
		// Clean up stale PID file if exists
		os.Remove(pidFilePath)
		return fmt.Errorf("service is not running")
	}

	content, _ := os.ReadFile(pidFilePath)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(content)))

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	// Force kill
	if err := process.Kill(); err != nil {
		return err
	}

	// Clean up PID file
	os.Remove(pidFilePath)
	fmt.Printf("Process %d terminated.\n", pid)
	return nil
}
