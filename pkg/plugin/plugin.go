// pkg/plugin/plugin.go
package plugin

import (
	"github.com/valyala/fasthttp"
)

type Plugin interface {
	Process(ctx *fasthttp.RequestCtx) (blocked bool, err error)
}

type InfoProvider interface {
	GetInfo() *PluginInfo
}

type Configurable interface {
	Configure(config map[string]string) error
}

type PluginInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
}

type AdvancedPlugin interface {
	Plugin
	InfoProvider
	Configurable

	Initialize() error
	Start() error
	Stop() error

	HealthCheck() error

	GetMetrics() map[string]interface{}
}
