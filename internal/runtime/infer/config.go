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
