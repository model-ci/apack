//go:generate easyjson -all config.go
package infer

import (
	"fmt"
	"path/filepath"
)

type Config struct {
	AutoRestart bool
	MaxRetries  int
	BasePath    string
	LogPath     string
	PidPath     string
}

func (c *Config) Validate() error {
	if c.BasePath == "" {
		return fmt.Errorf("base path cannot be empty")
	}

	if c.LogPath == "" {
		c.LogPath = filepath.Join(c.BasePath, "logs")
	}

	if c.PidPath == "" {
		c.PidPath = filepath.Join(c.BasePath, "pids")
	}

	return nil
}

type Params struct {
	ID             string
	ExecutablePath string
	ModelPath      string
	LogFile        string
	PidFile        string // Required for IsRunning check
	Port           int
	Threads        int
	GpuLayers      int
	CtxSize        int
	Verbose        bool
	Reference      string
}

func (p *Params) Validate() error {
	if p.ModelPath == "" {
		return fmt.Errorf("model path cannot be empty")
	}

	if p.ID == "" {
		return fmt.Errorf("id cannot be empty")
	}

	if p.LogFile == "" {
		return fmt.Errorf("log file cannot be empty")
	}

	if p.PidFile == "" {
		return fmt.Errorf("pid file cannot be empty")
	}

	if p.Port == 0 {
		return fmt.Errorf("port cannot be zero")
	}

	if p.Threads == 0 {
		p.Threads = 2
	}

	if p.GpuLayers < 0 {
		p.GpuLayers = 0
	}

	if p.CtxSize == 0 {
		p.CtxSize = 4096
	}

	return nil
}
