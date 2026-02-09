package infer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/types"
)

type LlamaCppInfer struct {
	*types.Params

	wg         sync.WaitGroup
	cmd        *exec.Cmd
	cancelFunc context.CancelFunc
}

func NewLlamaCppInfer(p *types.Params) (Proc, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &LlamaCppInfer{
		Params: p,
	}, nil
}

// Exec starts the process as a detached daemon.
func (l *LlamaCppInfer) Exec(ctx context.Context) error {
	// 1. Check if the service is already running
	if l.PidFile != "" {
		if isRunning(l.PidFile) {
			return fmt.Errorf("service appears to be already running (check PID file: %s)", l.PidFile)
		}
		// Clean up stale PID file if process is dead
		_ = os.Remove(l.PidFile)
	}

	// 2. Prepare log file (pass file descriptor directly to child process)
	var logFile *os.File
	if l.LogFile != "" {
		if err := ensureDir(l.LogFile); err != nil {
			return err
		}
		// Open log file in append mode
		f, err := os.OpenFile(l.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}

		// Write session start timestamp
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		f.WriteString(fmt.Sprintf("\n--- [%s] Process Started (Detached) ---\n", timestamp))

		logFile = f
		// Parent process can close the handle after Start(), child inherits a copy
		defer logFile.Close()
	}

	// 3. Prepare PID directory
	if l.PidFile != "" {
		if err := ensureDir(l.PidFile); err != nil {
			return err
		}
	}

	// 4. Construct arguments
	args := []string{
		"-m", l.ModelPath,
		"--port", fmt.Sprintf("%d", l.Port),
		"-c", fmt.Sprintf("%d", l.CtxSize),
		"-t", fmt.Sprintf("%d", l.Threads),
		"-ngl", fmt.Sprintf("%d", l.GpuLayers),
	}

	if !l.Verbose {
		args = append(args, "--log-disable")
	}

	if l.Embeddings {
		args = append(args, "--embeddings")
	}

	exePath := l.ExecutablePath
	if exePath == "" {
		//cwd, _ := os.Getwd()
		//exePath = filepath.Join(cwd, GetDefaultExecutableName())
		exePath = GetDefaultExecutableName()
	}

	// Use exec.Command instead of CommandContext to decouple child process lifetime
	// from the parent's context. Parent exit won't kill the child.
	cmd := exec.Command(exePath, args...)

	// Set environment variables
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"USER=" + os.Getenv("USER"),
		"LANG=" + os.Getenv("LANG"),
		"LC_ALL=C",
		"TMPDIR=/tmp",
		"CGO_ENABLED=1",
		"CC=clang",
		"CXX=clang++",
		"SHELL=/bin/sh",
	}

	// Create a new session group (Setsid) to detach from the terminal
	cmd.SysProcAttr = getSysProcAttr()

	// Direct IO redirection to avoid pipe dependency
	// If parent exits, pipes would close and might affect child.
	// Using file handles avoids this issue.
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		// Redirect to /dev/null if no log file is provided
		cmd.Stdout = nil
		cmd.Stderr = nil
	}

	log.Logger.Debugf("Starting detached process: %s", exePath)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %v", err)
	}

	// 5. Write PID to file
	pid := cmd.Process.Pid
	if l.PidFile != "" {
		if err := os.WriteFile(l.PidFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
			log.Logger.Warnf("failed to write PID file: %v", err)
		}
	}

	// 6. Async monitor (Reap zombie process + Cleanup PID file)
	go func() {
		// Wait blocks until the child process truly exits.
		// If the parent process exits first, this goroutine stops,
		// so os.Remove won't execute, preserving the PID file for the running child.
		_ = cmd.Wait()

		// Code reaches here ONLY if child process has exited.
		// Safe to remove the PID file now.
		if l.PidFile != "" {
			_ = os.Remove(l.PidFile)
		}
	}()

	return nil
}

func (l *LlamaCppInfer) IsRunning() bool {
	return isRunning(l.PidFile)
}

func (l *LlamaCppInfer) Kill(context.Context) error {
	return killFromPidFile(l.PidFile)
}

// Helper: Ensure directory exists
func ensureDir(filePath string) error {
	return os.MkdirAll(filepath.Dir(filePath), 0755)
}

// Helper: Stream output to writer
func streamOutput(r io.Reader, label string, fileWriter io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		text := scanner.Text()
		if fileWriter != nil {
			fileWriter.Write([]byte(text + "\n"))
		}
	}
}
