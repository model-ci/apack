package proc

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/pkg/infer/llama/config"
	"github.com/model-ci/apack/pkg/infer/llama/types"
)

type Server struct {
	bin     string
	config  *config.ServerConfig
	cmd     *exec.Cmd
	apiPort int
	running bool
	client  *http.Client
}

func (s *Server) IsRunning() bool {
	return s.running
}

func NewServer(config *config.ServerConfig, path string) (*Server, error) {
	return &Server{
		bin:     path,
		config:  config,
		apiPort: config.Port,
		client: &http.Client{
			Timeout: config.RequestTimeout,
		},
	}, nil
}

func writePIDFile(pidFilePath string, pid int) error {
	dir := filepath.Dir(pidFilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("error creating directory %s: %w", dir, err)
	}

	pidData := []byte(fmt.Sprintf("%d", pid))
	if err := os.WriteFile(pidFilePath, pidData, 0644); err != nil {
		return fmt.Errorf("error writing PID to file %s: %w", pidFilePath, err)
	}

	return nil
}

func (s *Server) Start(ctx context.Context) error {
	if s.running {
		return fmt.Errorf("server already running")
	}

	binPath, err := filepath.Abs(s.bin)
	if err != nil {
		return err
	}

	binDir := filepath.Dir(binPath)
	assetsDir := filepath.Join(binDir, "assets")
	assetsAbsDir, err := filepath.Abs(assetsDir)
	if err != nil {
		return err
	}

	log.Logger.Debugf("model path is %s", s.config.ModelPath)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command(
			binPath,
			"--server",
			"--model", s.config.ModelPath,
			"--host", s.config.Host,
			"--port", fmt.Sprintf("%d", s.config.Port),
			"--gpu", "AUTO",
			"--nobrowser",
			"--unsecure",
		)
	} else {
		absModelPath, err := filepath.Abs(s.config.ModelPath)
		if err != nil {
			return err
		}
		cmd = exec.Command("sh", "-c",
			fmt.Sprintf("%s --server --model %s --host %s --port %d --ctx-size %d --path %s --gpu AUTO --nobrowser --unsecure",
				binPath, absModelPath, s.config.Host, s.config.Port, s.config.ContextSize, assetsAbsDir),
		)
	}

	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"USER=" + os.Getenv("USER"),
		"LANG=" + os.Getenv("LANG"),
		"LC_ALL=C",
		"TMPDIR=/tmp",
		"CGO_ENABLED=1",
		"LLAMAFILE_TMPDIR=/tmp",
		"LLAMAFILE_DISABLE_JIT=1",
		"CC=clang",
		"CXX=clang++",
		"SHELL=/bin/sh",
		"CLI_CONTEXT=",
		"URFAVE_CLI_VERSION=",
		"COBRA_SILENCE_USAGE=",
	}

	SetProcessGroup(cmd)

	workDir := filepath.Join(binDir, filepath.Dir(s.config.ModelPath))
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}

	logFile := filepath.Join(workDir, "log")
	cmd.Dir = binDir
	logs, err := os.OpenFile(logFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file for runtime: %w", err)
	}

	defer func() {
		if errClose := logs.Close(); errClose != nil {
			if err == nil {
				err = fmt.Errorf("failed to close log file: %w", errClose)
			} else {
				err = fmt.Errorf("%v; failed to close log file: %w", err, errClose)
			}
		}
	}()

	log.Logger.Debugf("Saving server logs to %s", logFile)
	cmd.Stdout = logs
	cmd.Stderr = logs

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting: %w", err)
	}

	pid := cmd.Process.Pid
	pidFile := filepath.Join(workDir, "pids")
	if err := writePIDFile(pidFile, pid); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	log.Logger.Debugf("Started with PID %d and saved to file.\n", pid)

	s.cmd = cmd
	s.running = true

	return nil
}

func (s *Server) Stop() error {
	if !s.running {
		return nil
	}

	if s.cmd != nil && s.cmd.Process != nil {
		if err := s.cmd.Process.Signal(syscall.SIGTERM); err == nil {
			done := make(chan error, 1)
			go func() {
				done <- s.cmd.Wait()
			}()

			select {
			case <-time.After(5 * time.Second):
				s.cmd.Process.Kill()
			case <-done:
			}
		} else {
			s.cmd.Process.Kill()
		}
	}

	s.running = false
	return nil
}

func (s *Server) IsHealthy() bool {
	if !s.running {
		return false
	}

	if s.cmd == nil || s.cmd.Process == nil {
		return false
	}

	err := s.cmd.Process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}

	resp, err := s.client.Get(fmt.Sprintf("http://%s:%d/health", s.config.Host, s.apiPort))
	if err != nil {
		return false
	}

	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func (s *Server) waitForReady(ctx context.Context) error {
	healthURL := fmt.Sprintf("http://%s:%d/health", s.config.Host, s.apiPort)

	for i := 0; i < 120; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			resp, err := s.client.Get(healthURL)
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(1 * time.Second)
		}
	}

	return fmt.Errorf("types failed to start within 2 minutes")
}

func ConvertLlamaToOpenAI(llamaResp *types.LlamaCompletionResponse) *types.CompletionResponse {
	return &types.CompletionResponse{
		ID:      fmt.Sprintf("cmpl-%d", time.Now().Unix()),
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   llamaResp.Model,
		Choices: []types.Choice{
			{
				Text:         llamaResp.Content,
				Index:        0,
				FinishReason: getFinishReason(llamaResp),
			},
		},
		Usage: types.Usage{
			PromptTokens:     llamaResp.TokensEvaluated,
			CompletionTokens: llamaResp.TokensPredicted,
			TotalTokens:      llamaResp.TokensEvaluated + llamaResp.TokensPredicted,
		},
	}
}

func getFinishReason(resp *types.LlamaCompletionResponse) string {
	if resp.StoppedEOS {
		return "stop"
	}
	if resp.StoppedLimit {
		return "length"
	}
	if resp.StoppedWord {
		return "stop"
	}
	return "stop"
}

func (s *Server) Complete(ctx context.Context, req *types.CompletionRequest) (*types.CompletionResponse, error) {
	url := fmt.Sprintf("http://%s:%d/completion", s.config.Host, s.apiPort)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var llamaResult types.LlamaCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&llamaResult); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return ConvertLlamaToOpenAI(&llamaResult), nil
}

func (s *Server) CompleteStream(ctx context.Context, req *types.CompletionRequest) (<-chan types.CompletionChunk, error) {
	url := fmt.Sprintf("http://%s:%d/v1/completions", s.config.Host, s.apiPort)

	req.Stream = true
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	ch := make(chan types.CompletionChunk, 10)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					break
				}

				var chunk types.CompletionChunk
				if err := json.Unmarshal([]byte(data), &chunk); err == nil {
					select {
					case ch <- chunk:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch, nil
}

func (s *Server) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
	url := fmt.Sprintf("http://%s:%d/v1/chat/completions", s.config.Host, s.apiPort)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result types.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (s *Server) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.ChatChunk, error) {
	url := fmt.Sprintf("http://%s:%d/v1/chat/completions", s.config.Host, s.apiPort)

	req.Stream = true
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	ch := make(chan types.ChatChunk, 10)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					break
				}

				var chunk types.ChatChunk
				if err := json.Unmarshal([]byte(data), &chunk); err == nil {
					select {
					case ch <- chunk:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch, nil
}

func (s *Server) Embed(ctx context.Context, req *types.EmbedRequest) (*types.EmbedResponse, error) {
	url := fmt.Sprintf("http://%s:%d/v1/embeddings", s.config.Host, s.apiPort)

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result types.EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (s *Server) Model(ctx context.Context) (*types.ModelListResponse, error) {
	resp, err := s.client.Get(fmt.Sprintf("http://%s:%d/v1/models", s.config.Host, s.apiPort))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result types.ModelListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
