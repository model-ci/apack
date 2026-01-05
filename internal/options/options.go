//go:generate easyjson options.go
package options

import (
	"fmt"
	"os"

	"github.com/model-ci/apack/internal/consts"
	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/utils"
	"github.com/urfave/cli/v2"
)

type Options struct {
	OptionConfig

	Ctx      *cli.Context
	Tmpdir   string
	BodySize int
}

func New(ctx *cli.Context) *Options {
	opts := Options{Ctx: ctx}

	configFile := ctx.String("config-file")
	oc, err := LoadOptionConfig(configFile)
	if err != nil {
		log.Logger.Warnf("Failed to load config file %s: %v", configFile, err)
		oc = &OptionConfig{}
	}

	// seq: cmdline-set > config-file > default-value
	opts.Endpoint = getStringValue(ctx, "endpoint", oc.Endpoint, consts.ApackSock())
	opts.Tmpdir = getStringValue(ctx, "tmp", oc.Tmp, "./tmp")
	opts.Log = getStringValue(ctx, "log", oc.Log, "")
	opts.LogLevel = getStringValue(ctx, "log-level", oc.LogLevel, "debug")
	opts.Root = getStringValue(ctx, "root", oc.Root, "/run/apackd")
	opts.DataDir = getStringValue(ctx, "data", oc.DataDir, "./data")
	opts.Cert = getStringValue(ctx, "cert", oc.Cert, "")
	opts.Key = getStringValue(ctx, "key", oc.Key, "")
	opts.Username = getStringValue(ctx, "username", oc.Username, "")
	opts.Password = getStringValue(ctx, "password", oc.Password, "")
	opts.Hosts = getStringSliceValue(ctx, "hosts", oc.Hosts, []string{consts.ApackSock()})

	bodyStr := getStringValue(ctx, "body", oc.Body, "256M")
	size, err := utils.ParseSize(bodyStr[0:len(bodyStr)-1], bodyStr[len(bodyStr)-1:])
	if err != nil {
		panic(err)
	}
	opts.BodySize = size
	opts.MemoryLimit = oc.MemoryLimit

	opts.GcPercent = getIntValue(ctx, "gc-percent", oc.GcPercent, 30)
	opts.ReadTimeout = getIntValue(ctx, "read-timeout", oc.ReadTimeout, 600)
	opts.WriteTimeout = getIntValue(ctx, "write-timeout", oc.WriteTimeout, 600)
	opts.IdleTimeout = getIntValue(ctx, "idle-timeout", oc.IdleTimeout, 1800)

	opts.Secure = getBoolValue(ctx, "secure", oc.Secure, false)
	opts.Auth = getBoolValue(ctx, "auth", oc.Auth, false)
	opts.Concurrency = getIntValue(ctx, "concurrency", oc.Concurrency, 1)
	opts.Compress = getIntValue(ctx, "compress", oc.Compress, 0)

	return &opts
}

func (o *Options) Validate() error {
	if o.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if o.Tmpdir == "" {
		return fmt.Errorf("tmp directory is required")
	}
	if o.BodySize <= 0 {
		return fmt.Errorf("body size is required")
	}
	if o.GcPercent < 10 || o.GcPercent > 100 {
		return fmt.Errorf("gc-percent must be between 10 and 100")
	}
	/*
		if o.MemoryLimit <= types.GB {
			return fmt.Errorf("memory-limit must be greater than 1GB")
		}
	*/
	return nil
}

func getStringSliceValue(ctx *cli.Context, flagName string, configValue []string, defaultValue []string) []string {
	if ctx.IsSet(flagName) {
		return ctx.StringSlice(flagName)
	}
	if len(configValue) > 0 {
		return configValue
	}
	return defaultValue
}

func getStringValue(ctx *cli.Context, flagName string, configValue string, defaultValue string) string {
	if ctx.IsSet(flagName) {
		return ctx.String(flagName)
	}
	if configValue != "" {
		return configValue
	}
	return defaultValue
}

func getIntValue(ctx *cli.Context, flagName string, configValue int, defaultValue int) int {
	if ctx.IsSet(flagName) {
		return ctx.Int(flagName)
	}
	if configValue != 0 {
		return configValue
	}
	return defaultValue
}

func getBoolValue(ctx *cli.Context, flagName string, configValue bool, defaultValue bool) bool {
	if ctx.IsSet(flagName) {
		return ctx.Bool(flagName)
	}
	return configValue || defaultValue
}

//easyjson:json
type OptionConfig struct {
	Endpoint     string   `json:"endpoint,omitempty"`
	Auth         bool     `json:"auth,omitempty"`
	Username     string   `json:"username,omitempty"`
	Password     string   `json:"password,omitempty"`
	Cert         string   `json:"cert,omitempty"`
	Key          string   `json:"key,omitempty"`
	Log          string   `json:"log,omitempty"`
	LogLevel     string   `json:"log-level,omitempty"`
	Root         string   `json:"root,omitempty"`
	DataDir      string   `json:"data-dir,omitempty"`
	Body         string   `json:"body,omitempty"`
	ReadTimeout  int      `json:"read-timeout,omitempty"`
	WriteTimeout int      `json:"write-timeout,omitempty"`
	IdleTimeout  int      `json:"idle-timeout,omitempty"`
	Tmp          string   `json:"tmp,omitempty"`
	GcPercent    int      `json:"gc-percent,omitempty"`
	MemoryLimit  int      `json:"memory-limit,omitempty"`
	Secure       bool     `json:"secure,omitempty"`
	Hosts        []string `json:"hosts,omitempty"`
	Concurrency  int      `json:"concurrency,omitempty"`
	Compress     int      `json:"compress,omitempty"`
}

func LoadOptionConfig(configFile string) (*OptionConfig, error) {
	oc := &OptionConfig{}

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return oc, nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	if err := oc.UnmarshalJSON(data); err != nil {
		return nil, err
	}

	return oc, nil
}
