package proxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/model-ci/apack/pkg/infer/llama/utils"
)

type ProxyServer struct {
	target       *url.URL
	reverseProxy *httputil.ReverseProxy
	config       *Config
}

type Config struct {
	TargetHost    string
	TargetPort    int
	Timeout       time.Duration
	MaxRetries    int
	BufferSize    int
	EnableLogging bool
	EnableMetrics bool
}

type ProxyMetrics struct {
	RequestsTotal  int64         `json:"requests_total"`
	ErrorsTotal    int64         `json:"errors_total"`
	AverageLatency time.Duration `json:"average_latency"`
	ActiveRequests int64         `json:"active_requests"`
}

func NewProxyServer(config *Config) (*ProxyServer, error) {
	target, err := url.Parse(fmt.Sprintf("http://%s:%d", config.TargetHost, config.TargetPort))
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	ps := &ProxyServer{
		target: target,
		config: config,
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		ps.modifyRequest(req)
	}

	proxy.ErrorHandler = ps.errorHandler

	proxy.ModifyResponse = ps.modifyResponse

	ps.reverseProxy = proxy

	return ps, nil
}

func (ps *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if ps.config.EnableLogging {
		utils.Info("Proxy request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
	}

	if ps.config.Timeout > 0 {
		ctx := r.Context()
		ctx, cancel := context.WithTimeout(ctx, ps.config.Timeout)
		defer cancel()
		r = r.WithContext(ctx)
	}

	if ps.isStreamRequest(r) {
		ps.handleStreamRequest(w, r)
		return
	}

	ps.reverseProxy.ServeHTTP(w, r)

	if ps.config.EnableLogging {
		duration := time.Since(start)
		utils.Info("Proxy response: %s %s took %v", r.Method, r.URL.Path, duration)
	}
}

func (ps *ProxyServer) modifyRequest(req *http.Request) {
	req.Header.Set("X-Forwarded-For", req.RemoteAddr)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Set("X-Forwarded-Host", req.Host)

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "go-llamafile-proxy/1.0")
	}

	req.Header.Del("Connection")
	req.Header.Del("Upgrade")
}

func (ps *ProxyServer) modifyResponse(resp *http.Response) error {
	resp.Header.Set("Access-Control-Allow-Origin", "*")
	resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	resp.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	resp.Header.Set("X-Proxied-By", "go-llamafile")

	return nil
}

func (ps *ProxyServer) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	if ps.config.EnableLogging {
		utils.Error("Proxy error for %s %s: %v", r.Method, r.URL.Path, err)
	}

	statusCode := http.StatusBadGateway
	message := "Service temporarily unavailable"

	if strings.Contains(err.Error(), "timeout") {
		statusCode = http.StatusGatewayTimeout
		message = "Request timeout"
	} else if strings.Contains(err.Error(), "connection refused") {
		statusCode = http.StatusServiceUnavailable
		message = "Service unavailable"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	fmt.Fprintf(w, `{"error": "%s", "code": %d}`, message, statusCode)
}

func (ps *ProxyServer) isStreamRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	accept := r.Header.Get("Accept")

	return strings.Contains(accept, "text/event-stream") ||
		strings.Contains(contentType, "text/event-stream") ||
		r.URL.Query().Get("stream") == "true"
}

func (ps *ProxyServer) handleStreamRequest(w http.ResponseWriter, r *http.Request) {
	targetURL := fmt.Sprintf("http://%s:%d%s", ps.config.TargetHost, ps.config.TargetPort, r.URL.Path)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	client := &http.Client{
		Timeout: 0,
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		utils.Error("Stream proxy request failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	w.WriteHeader(resp.StatusCode)

	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, resp.Body)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintf(w, "%s\n", line)
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		utils.Error("Stream scanning error: %v", err)
	}
}

type LoadBalancer struct {
	targets []*ProxyServer
	current int
}

func NewLoadBalancer(configs []*Config) (*LoadBalancer, error) {
	var targets []*ProxyServer

	for _, config := range configs {
		proxy, err := NewProxyServer(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy for %s:%d: %w",
				config.TargetHost, config.TargetPort, err)
		}
		targets = append(targets, proxy)
	}

	return &LoadBalancer{
		targets: targets,
	}, nil
}

func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(lb.targets) == 0 {
		http.Error(w, "No available targets", http.StatusServiceUnavailable)
		return
	}

	target := lb.targets[lb.current]
	lb.current = (lb.current + 1) % len(lb.targets)

	target.ServeHTTP(w, r)
}

type HealthChecker struct {
	targets    []*ProxyServer
	interval   time.Duration
	timeout    time.Duration
	healthPath string
}

func NewHealthChecker(targets []*ProxyServer, interval, timeout time.Duration, healthPath string) *HealthChecker {
	return &HealthChecker{
		targets:    targets,
		interval:   interval,
		timeout:    timeout,
		healthPath: healthPath,
	}
}

func (hc *HealthChecker) Start() {
	ticker := time.NewTicker(hc.interval)
	go func() {
		for range ticker.C {
			hc.checkHealth()
		}
	}()
}

func (hc *HealthChecker) checkHealth() {
	for _, target := range hc.targets {
		go hc.checkTarget(target)
	}
}

func (hc *HealthChecker) checkTarget(target *ProxyServer) {
	url := fmt.Sprintf("%s%s", target.target.String(), hc.healthPath)

	client := &http.Client{Timeout: hc.timeout}
	resp, err := client.Get(url)
	if err != nil {
		utils.Warn("Health check failed for %s: %v", target.target.String(), err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		utils.Debug("Health check passed for %s", target.target.String())
	} else {
		utils.Warn("Health check failed for %s: status %d", target.target.String(), resp.StatusCode)
	}
}
