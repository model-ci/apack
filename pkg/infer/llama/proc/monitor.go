package proc

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/model-ci/apack/pkg/infer/llama/config"
)

type Monitor struct {
	c  *config.Config
	sc *config.ServerConfig

	server *Server

	mu         sync.RWMutex
	isHealthy  bool
	lastCheck  time.Time
	checkCount int64
	errorCount int64

	checkInterval time.Duration
	timeout       time.Duration
	maxErrors     int64

	stopChan chan struct{}
	stopped  bool
}

type HealthMetrics struct {
	IsHealthy    bool          `json:"is_healthy"`
	LastCheck    time.Time     `json:"last_check"`
	CheckCount   int64         `json:"check_count"`
	ErrorCount   int64         `json:"error_count"`
	ErrorRate    float64       `json:"error_rate"`
	ResponseTime time.Duration `json:"response_time"`
}

func NewMonitor(server *Server, c *config.Config, sc *config.ServerConfig) *Monitor {
	return &Monitor{
		server:        server,
		c:             c,
		sc:            sc,
		checkInterval: 30 * time.Second,
		timeout:       5 * time.Second,
		maxErrors:     5,
		stopChan:      make(chan struct{}),
	}
}

func (m *Monitor) Start() {
	m.mu.Lock()
	if !m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = false
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	go m.monitorLoop()
}

func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.stopped {
		m.stopped = true
		close(m.stopChan)
	}
}

func (m *Monitor) monitorLoop() {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	m.performHealthCheck()

	for {
		select {
		case <-ticker.C:
			m.performHealthCheck()
		case <-m.stopChan:
			return
		}
	}
}

func (m *Monitor) performHealthCheck() {
	m.mu.Lock()
	m.checkCount++
	m.lastCheck = time.Now()
	m.mu.Unlock()

	start := time.Now()
	healthy := m.checkServerHealth()
	responseTime := time.Since(start)

	m.mu.Lock()
	defer m.mu.Unlock()

	if !healthy {
		m.errorCount++
		m.isHealthy = false

		if m.errorCount >= m.maxErrors && m.c.AutoRestart {
			go m.attemptRestart()
		}
	} else {
		m.isHealthy = true
		if m.errorCount > 0 {
			m.errorCount = 0
		}
	}

	_ = responseTime
}

func (m *Monitor) checkServerHealth() bool {
	if !m.server.IsHealthy() {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	healthURL := fmt.Sprintf("http://%s:%d/health", m.sc.Host, m.sc.Port)

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: m.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func (m *Monitor) attemptRestart() {
	m.mu.Lock()
	if m.errorCount < m.maxErrors {
		m.mu.Unlock()
		return
	}
	m.errorCount = 0
	m.mu.Unlock()

	fmt.Printf("️Service unhealthy detected, attempting restart...\n")

	ctx, cancel := context.WithTimeout(context.Background(), m.sc.StartTimeout)
	defer cancel()

	if err := m.server.Stop(); err != nil {
		fmt.Printf("Failed to stop service: %v\n", err)
		return
	}

	time.Sleep(2 * time.Second)

	if err := m.server.Start(ctx); err != nil {
		fmt.Printf("Failed to restart service: %v\n", err)
		return
	}

	fmt.Printf("Service restarted successfully\n")
}

func (m *Monitor) GetMetrics() *HealthMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errorRate float64
	if m.checkCount > 0 {
		errorRate = float64(m.errorCount) / float64(m.checkCount)
	}

	return &HealthMetrics{
		IsHealthy:  m.isHealthy,
		LastCheck:  m.lastCheck,
		CheckCount: m.checkCount,
		ErrorCount: m.errorCount,
		ErrorRate:  errorRate,
	}
}

func (m *Monitor) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isHealthy
}
