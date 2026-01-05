package config

import (
	"fmt"

	"github.com/model-ci/apack/internal/options"
	"github.com/model-ci/apack/internal/types"
)

type APIConfig struct {
	opts             *options.Options
	Base             *BaseConfig
	Components       []string
	ComponentModules map[string][]string
}

func NewAPIConfig(opts *options.Options) *APIConfig {
	ac := &APIConfig{opts: opts, ComponentModules: make(map[string][]string)}
	ac.Components = append(ac.Components, types.Base)
	return ac
}

func (c *APIConfig) Validate() error {
	switch c.opts.Ctx.Command.Name {
	case types.Daemon:
		if c.Base == nil {
			return fmt.Errorf("base config is required")
		}
		c.ComponentModules[types.Base] = c.Base.Modules
	}
	if len(c.ComponentModules) <= 0 {
		return fmt.Errorf("at least one component modules is required: %s", c.opts.Ctx.Command.Name)
	}
	return nil
}

func (c *APIConfig) GetDataDir() string {
	return c.opts.DataDir
}

func (c *APIConfig) GetHosts() []string {
	return c.opts.Hosts
}

func (c *APIConfig) GetKeyFile() string {
	return c.opts.Key
}

func (c *APIConfig) GetCertFile() string {
	return c.opts.Cert
}

func (c *APIConfig) GetEndpoint() string {
	return c.opts.Endpoint
}

func (c *APIConfig) GetRoot() string {
	return c.opts.Root
}

func (c *APIConfig) GetReadTimeout() int {
	return c.opts.ReadTimeout
}

func (c *APIConfig) GetWriteTimeout() int {
	return c.opts.WriteTimeout
}

func (c *APIConfig) GetIdleTimeout() int {
	return c.opts.IdleTimeout
}

type BaseConfig struct {
	Opts    *options.Options
	Modules []string
}

func NewBase(opts *options.Options) *BaseConfig {
	bc := &BaseConfig{Opts: opts}
	bc.Modules = opts.Ctx.StringSlice("mod")
	return bc
}

func (c *BaseConfig) Validate() error {
	if len(c.Modules) == 0 {
		return fmt.Errorf("at least one module is required")
	}
	return nil
}
