//go:generate easyjson config.go
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/model-ci/apack/pkg/infer/llama/errs"
)

type Config struct {
	AutoRestart bool   `json:"auto_restart"`
	MaxRetries  int    `json:"max_retries"`
	BinaryPath  string `json:"binary_path,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		AutoRestart: true,
		MaxRetries:  3,
	}
}

func (c *Config) Validate() error {
	return nil
}

func (c *Config) SetDefaults() {
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
}

type ServerConfig struct {
	ModelPath      string        `json:"model_path"`
	Host           string        `json:"host"`
	Port           int           `json:"port"`
	ContextSize    int           `json:"context_size"`
	GPULayers      int           `json:"gpu_layers"`
	Threads        int           `json:"threads"`
	StartTimeout   time.Duration `json:"start_timeout"`
	RequestTimeout time.Duration `json:"request_timeout"`
	LogLevel       string        `json:"log_level"`
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:           "127.0.0.1",
		Port:           8081,
		ContextSize:    16384,
		GPULayers:      -1,
		Threads:        0,
		StartTimeout:   60 * time.Second,
		RequestTimeout: 30 * time.Second,
		LogLevel:       "info",
	}
}

func (c *ServerConfig) Validate() error {
	if c.ModelPath == "" {
		return errs.ErrInvalidModelPath
	}

	if _, err := os.Stat(c.ModelPath); os.IsNotExist(err) {
		return errs.ErrModelNotFound
	}

	if c.Port <= 0 || c.Port > 65535 {
		return errs.ErrInvalidPort
	}

	return nil
}

func (c *ServerConfig) SetDefaults() {
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Port == 0 {
		c.Port = 8081
	}

	if c.ContextSize == 0 {
		c.ContextSize = 16384
	}

	if c.StartTimeout == 0 {
		c.StartTimeout = 60 * time.Second
	}

	if c.RequestTimeout == 0 {
		c.RequestTimeout = 30 * time.Second
	}

	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
}

type APIConfig struct {
	Enabled     bool   `json:"enabled"`
	Port        int    `json:"port"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	Theme       string `json:"theme"`
	AuthEnabled bool   `json:"auth_enabled"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

func DefaultAPIConfig() *APIConfig {
	return &APIConfig{
		Port:     8080,
		Title:    "ApackAI",
		Theme:    "dark",
		Username: "admin",
		Password: "nimda",
	}
}

func (c *APIConfig) Validate() error {
	if c.Enabled && (c.Port <= 0 || c.Port > 65535) {
		return fmt.Errorf("invalid UI port: %d", c.Port)
	}

	return nil
}

func (c *APIConfig) SetDefaults() {
	if c.Port == 0 {
		c.Port = 8080
	}

	if c.Title == "" {
		c.Title = "ApackAI"
	}

	if c.Theme == "" {
		c.Theme = "dark"
	}

	if c.Username == "" {
		c.Username = "admin"
	}

	if c.Password == "" {
		c.Password = "nimda"
	}
}

//easyjson:json
type RuntimeConfig struct {
	ServerConfig *ServerConfig `json:"server_config"`
	APIConfig    *APIConfig    `json:"api_config"`
}
