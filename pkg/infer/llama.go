package infer

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/model-ci/apack/pkg/infer/llama/api"
	"github.com/model-ci/apack/pkg/infer/llama/config"
	"github.com/model-ci/apack/pkg/infer/llama/errs"
	"github.com/model-ci/apack/pkg/infer/llama/llamafile"
	"github.com/model-ci/apack/pkg/infer/llama/proc"
	"github.com/model-ci/apack/pkg/infer/llama/types"
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

type Infer interface {
	Create(*config.ServerConfig, *config.APIConfig, string) (types.Runner, oci.Descriptor, error)
	Get(id string) (types.Runner, error)
	Close() error
}

type client struct {
	cfg       *config.Config
	binaryMgr *llamafile.Manager
	runners   map[string]types.Runner
}

func New(cfg *config.Config) (Infer, error) {
	c := &client{
		cfg:     cfg,
		runners: map[string]types.Runner{},
	}

	err := os.MkdirAll(cfg.BinaryPath, 0o755)
	if err != nil {
		return nil, err
	}

	c.binaryMgr, err = llamafile.NewManager(cfg.BinaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create binary manager: %w", err)
	}
	return c, nil
}

func (c *client) Create(sc *config.ServerConfig, ac *config.APIConfig, ref string) (types.Runner, oci.Descriptor, error) {
	if sc == nil {
		sc = config.DefaultServerConfig()
	} else {
		sc.SetDefaults()
	}

	if err := sc.Validate(); err != nil {
		return nil, oci.DescriptorEmptyJSON, fmt.Errorf("invalid server config: %w", err)
	}

	if ac == nil {
		ac = config.DefaultAPIConfig()
	} else {
		ac.SetDefaults()
	}

	if err := ac.Validate(); err != nil {
		return nil, oci.DescriptorEmptyJSON, fmt.Errorf("invalid api config: %w", err)
	}

	binPath, err := c.binaryMgr.GetBinaryPath()
	if err != nil {
		return nil, oci.DescriptorEmptyJSON, fmt.Errorf("failed to get binary path: %w", err)
	}

	runner, err := create(sc, *ac, binPath)
	if err != nil {
		return nil, oci.DescriptorEmptyJSON, err
	}

	rc := &config.RuntimeConfig{
		ServerConfig: sc,
		APIConfig:    ac,
	}

	configBytes, err := rc.MarshalJSON()
	if err != nil {
		return nil, oci.DescriptorEmptyJSON, err
	}

	dgt := digest.FromBytes(configBytes)
	c.runners[dgt.Encoded()[:12]] = runner

	return runner, oci.Descriptor{
		MediaType: oci.MediaTypeDescriptor,
		Digest:    dgt,
		Size:      int64(len(configBytes)),
		Data:      configBytes,
		Annotations: map[string]string{
			layerdb.OCIAnnotationRefName: ref,
		},
	}, nil
}

func (c *client) Get(id string) (types.Runner, error) {
	r, ok := c.runners[id]
	if !ok {
		return nil, fmt.Errorf("runner not found")
	}
	return r, nil
}

func (c *client) Close() error {
	err := c.binaryMgr.Cleanup()
	if err != nil {
		return err
	}

	return nil
}

type runner struct {
	sc *config.ServerConfig
	ac config.APIConfig

	procServer *proc.Server
	apiServer  *api.Server

	mu            sync.RWMutex
	running       bool
	startTime     time.Time
	eventHandlers []types.EventHandler

	metrics   *types.Metrics
	metricsMu sync.RWMutex
}

func create(sc *config.ServerConfig, ac config.APIConfig, workspace string) (types.Runner, error) {
	procServer, err := proc.NewServer(sc, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to create proc server: %w", err)
	}

	cli := &runner{
		sc:            sc,
		ac:            ac,
		procServer:    procServer,
		metrics:       &types.Metrics{},
		eventHandlers: make([]types.EventHandler, 0),
	}

	if ac.Enabled {
		apiServer, err := api.NewServer(&ac, cli)
		if err != nil {
			return nil, fmt.Errorf("failed to create api server: %w", err)
		}
		cli.apiServer = apiServer
		if err := cli.apiServer.FileDump(workspace); err != nil {
			return nil, err
		}
	}

	return cli, nil
}

func (r *runner) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return errs.ErrAlreadyRunning
	}

	if err := r.procServer.Start(ctx); err != nil {
		r.emitEvent(types.EventError, fmt.Sprintf("Failed to start proc server: %v", err), nil)
		return fmt.Errorf("failed to start proc server: %w", err)
	}

	r.running = true
	r.startTime = time.Now()

	r.emitEvent(types.EventStarted, "Client started successfully", map[string]interface{}{
		"model": r.sc.ModelPath,
		"host":  r.sc.Host,
		"port":  r.sc.Port,
	})

	return nil
}

func (r *runner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return errs.ErrNotRunning
	}

	if r.apiServer != nil {
		r.apiServer.Stop()
	}

	if err := r.procServer.Stop(); err != nil {
		r.emitEvent(types.EventError, fmt.Sprintf("Failed to stop proc server: %v", err), nil)
		return fmt.Errorf("failed to stop proc server: %w", err)
	}

	r.running = false

	r.emitEvent(types.EventStopped, "Client stopped successfully", nil)

	return nil
}

func (r *runner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running && r.procServer.IsHealthy()
}

func (r *runner) Restart() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		if err := r.procServer.Stop(); err != nil {
			return fmt.Errorf("failed to stop proc server: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.sc.StartTimeout)
	defer cancel()

	if err := r.procServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to restart proc server: %w", err)
	}

	r.running = true

	r.emitEvent(types.EventRestarted, "Client restarted successfully", nil)

	return nil
}

func (r *runner) Complete(ctx context.Context, req *types.CompletionRequest) (*types.CompletionResponse, error) {
	if !r.IsRunning() {
		return nil, errs.ErrNotRunning
	}

	start := time.Now()
	defer func() {
		r.updateMetrics(time.Since(start), 1)
	}()

	return r.procServer.Complete(ctx, req)
}

func (r *runner) CompleteStream(ctx context.Context, req *types.CompletionRequest) (<-chan types.CompletionChunk, error) {
	if !r.IsRunning() {
		return nil, errs.ErrNotRunning
	}

	req.Stream = true
	return r.procServer.CompleteStream(ctx, req)
}

func (r *runner) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
	if !r.IsRunning() {
		return nil, errs.ErrNotRunning
	}

	start := time.Now()
	defer func() {
		r.updateMetrics(time.Since(start), 1)
	}()

	return r.procServer.Chat(ctx, req)
}

func (r *runner) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.ChatChunk, error) {
	if !r.IsRunning() {
		return nil, errs.ErrNotRunning
	}

	req.Stream = true
	return r.procServer.ChatStream(ctx, req)
}

func (r *runner) Embed(ctx context.Context, req *types.EmbedRequest) (*types.EmbedResponse, error) {
	if !r.IsRunning() {
		return nil, errs.ErrNotRunning
	}

	start := time.Now()
	defer func() {
		r.updateMetrics(time.Since(start), 1)
	}()

	return r.procServer.Embed(ctx, req)
}

func (r *runner) Model(ctx context.Context) (*types.ModelListResponse, error) {
	if !r.IsRunning() {
		return nil, errs.ErrNotRunning
	}

	start := time.Now()
	defer func() {
		r.updateMetrics(time.Since(start), 1)
	}()

	return r.procServer.Model(ctx)
}

func (r *runner) StartAPI(ctx context.Context) error {
	if r.apiServer == nil {
		return errs.ErrUINotEnabled
	}

	if err := r.apiServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start api server: %w", err)
	}

	r.emitEvent(types.EventUIStarted, "api server started", map[string]interface{}{
		"url": r.GetApiURL(),
	})

	return nil
}

func (r *runner) StopAPI() error {
	if r.apiServer == nil {
		return nil
	}

	if err := r.apiServer.Stop(); err != nil {
		return fmt.Errorf("failed to stop UI server: %w", err)
	}

	r.emitEvent(types.EventUIStopped, "UI server stopped", nil)

	return nil
}

func (r *runner) IsAPIRunning() bool {
	if r.apiServer == nil {
		return false
	}

	return r.apiServer.IsRunning()
}

func (r *runner) GetApiURL() string {
	if r.apiServer == nil {
		return ""
	}

	return r.apiServer.GetURL()
}

func (r *runner) Health() (*types.HealthStatus, error) {
	status := &types.HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Uptime:    time.Since(r.startTime),
		Services:  make(map[string]string),
	}

	if r.IsRunning() {
		status.Services["llamafile"] = "healthy"
	} else {
		status.Services["llamafile"] = "unhealthy"
		status.Status = "unhealthy"
	}

	if r.apiServer != nil {
		if r.IsAPIRunning() {
			status.Services["api"] = "healthy"
		} else {
			status.Services["api"] = "unhealthy"
		}
	}

	return status, nil
}

func (r *runner) Metrics() (*types.Metrics, error) {
	r.metricsMu.RLock()
	defer r.metricsMu.RUnlock()

	metrics := *r.metrics
	return &metrics, nil
}

func (r *runner) OnEvent(handler types.EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eventHandlers = append(r.eventHandlers, handler)
}

func (r *runner) emitEvent(eventType types.EventType, message string, data interface{}) {
	event := types.Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Message:   message,
		Data:      data,
	}

	for _, handler := range r.eventHandlers {
		go handler(event)
	}
}

func (r *runner) updateMetrics(latency time.Duration, requests int64) {
	r.metricsMu.Lock()
	defer r.metricsMu.Unlock()

	r.metrics.RequestsTotal += requests

	if r.metrics.RequestsTotal == 1 {
		r.metrics.AverageLatency = latency
	} else {
		r.metrics.AverageLatency = time.Duration(
			(int64(r.metrics.AverageLatency)*r.metrics.RequestsTotal + int64(latency)) /
				(r.metrics.RequestsTotal + 1),
		)
	}

	uptime := time.Since(r.startTime)
	if uptime > 0 {
		r.metrics.RequestsPerSecond = float64(r.metrics.RequestsTotal) / uptime.Seconds()
	}
}
