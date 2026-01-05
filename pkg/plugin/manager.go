// pkg/plugin/manager.go
package plugin

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/valyala/fasthttp"
)

type Manager struct {
	plugins map[string]*PluginClient
	mu      sync.RWMutex
	config  *ManagerConfig
}

type PluginClient struct {
	name      string
	client    *plugin.Client
	rpcClient plugin.ClientProtocol
	impl      Plugin
	info      *PluginInfo
	Status    PluginStatus
	StartTime time.Time
}

type PluginStatus int

const (
	PluginStatusStopped PluginStatus = iota
	PluginStatusStarting
	PluginStatusRunning
	PluginStatusError
)

func (s PluginStatus) String() string {
	switch s {
	case PluginStatusStopped:
		return "stopped"
	case PluginStatusStarting:
		return "starting"
	case PluginStatusRunning:
		return "running"
	case PluginStatusError:
		return "error"
	default:
		return "unknown"
	}
}

type ManagerConfig struct {
	PluginDir     string        `yaml:"plugin_dir"`
	Timeout       time.Duration `yaml:"timeout"`
	MaxRetries    int           `yaml:"max_retries"`
	HealthCheck   bool          `yaml:"health_check"`
	CheckInterval time.Duration `yaml:"check_interval"`
}

func NewManager(config *ManagerConfig) *Manager {
	if config == nil {
		config = &ManagerConfig{
			PluginDir:     "./plugins",
			Timeout:       30 * time.Second,
			MaxRetries:    3,
			HealthCheck:   true,
			CheckInterval: 30 * time.Second,
		}
	}

	return &Manager{
		plugins: make(map[string]*PluginClient),
		config:  config,
	}
}

func (m *Manager) LoadPlugin(name string) error {
	return m.LoadPluginWithConfig(name, nil)
}

func (m *Manager) LoadPluginWithConfig(name string, config map[string]string) error {
	pluginPath := fmt.Sprintf("%s/%s", m.config.PluginDir, name)

	logger := hclog.New(&hclog.LoggerOptions{
		Name:   fmt.Sprintf("plugin-%s", name),
		Level:  hclog.Info,
		Output: os.Stdout,
	})

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  handshakeConfig,
		Plugins:          pluginMap,
		Cmd:              exec.Command(pluginPath),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		Logger:           logger,
	})

	m.mu.Lock()
	pluginClient := &PluginClient{
		name:      name,
		client:    client,
		Status:    PluginStatusStarting,
		StartTime: time.Now(),
	}
	m.plugins[name] = pluginClient
	m.mu.Unlock()

	rpcClient, err := client.Client()
	if err != nil {
		m.updatePluginStatus(name, PluginStatusError)
		return fmt.Errorf("failed to get RPC client: %w", err)
	}

	raw, err := rpcClient.Dispense("plugin")
	if err != nil {
		m.updatePluginStatus(name, PluginStatusError)
		return fmt.Errorf("failed to dispense plugin: %w", err)
	}

	impl := raw.(Plugin)

	m.mu.Lock()
	pluginClient.rpcClient = rpcClient
	pluginClient.impl = impl
	pluginClient.Status = PluginStatusRunning
	m.mu.Unlock()

	if infoProvider, ok := impl.(InfoProvider); ok {
		info := infoProvider.GetInfo()
		m.mu.Lock()
		pluginClient.info = info
		m.mu.Unlock()
		log.Printf("Loaded plugin: %s v%s - %s", info.Name, info.Version, info.Description)
	}

	if config != nil {
		if configurable, ok := impl.(Configurable); ok {
			if err := configurable.Configure(config); err != nil {
				log.Printf("Failed to configure plugin %s: %v", name, err)
			}
		}
	}

	if m.config.HealthCheck {
		go m.healthCheckRoutine(name)
	}

	log.Printf("Plugin %s loaded successfully", name)
	return nil
}

func (m *Manager) GetPlugin(name string) Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if client, exists := m.plugins[name]; exists && client.Status == PluginStatusRunning {
		return client.impl
	}
	return nil
}

func (m *Manager) GetPluginInfo(name string) *PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if client, exists := m.plugins[name]; exists {
		return client.info
	}
	return nil
}

func (m *Manager) ListPlugins() map[string]*PluginClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*PluginClient)
	for name, client := range m.plugins {
		result[name] = client
	}
	return result
}

func (m *Manager) UnloadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	if client.client != nil {
		client.client.Kill()
	}

	delete(m.plugins, name)
	log.Printf("Plugin %s unloaded", name)
	return nil
}

func (m *Manager) ProcessRequest(ctx *fasthttp.RequestCtx, pluginNames []string) (bool, error) {
	for _, name := range pluginNames {
		plugin := m.GetPlugin(name)
		if plugin == nil {
			continue
		}

		blocked, err := plugin.Process(ctx)
		if err != nil {
			log.Printf("Plugin %s error: %v", name, err)
			continue
		}

		if blocked {
			return true, nil
		}
	}

	return false, nil
}

func (m *Manager) updatePluginStatus(name string, status PluginStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, exists := m.plugins[name]; exists {
		client.Status = status
	}
}

func (m *Manager) healthCheckRoutine(name string) {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.RLock()
		client, exists := m.plugins[name]
		m.mu.RUnlock()

		if !exists {
			return
		}

		if client.Status != PluginStatusRunning {
			continue
		}

		if advanced, ok := client.impl.(AdvancedPlugin); ok {
			if err := advanced.HealthCheck(); err != nil {
				log.Printf("Plugin %s health check failed: %v", name, err)
				m.updatePluginStatus(name, PluginStatusError)
			}
		}
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.plugins {
		if client.client != nil {
			client.client.Kill()
		}
		log.Printf("Plugin %s closed", name)
	}

	m.plugins = make(map[string]*PluginClient)
}

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "APACK_PLUGIN",
	MagicCookieValue: "apack",
}

var pluginMap = map[string]plugin.Plugin{
	"plugin": &PluginGRPC{},
}
