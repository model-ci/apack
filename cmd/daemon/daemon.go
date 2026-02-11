package daemon

import (
	"fmt"
	"os"
	"reflect"

	"github.com/model-ci/apack/internal/api"
	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/config"
	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/options"
	"github.com/model-ci/apack/internal/program"
	dm "github.com/sevlyar/go-daemon"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:        "daemon",
	Usage:       "The backend of a continuous integration tool for an AI model",
	UsageText:   ``,
	ArgsUsage:   "",
	Description: "",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config-file",
			Aliases: []string{"cf"},
			Value:   "/etc/apack/daemon.json",
			Usage:   "daemon configuration file ",
			EnvVars: []string{"APACK_CONFIG_FILE"},
		},
		&cli.StringSliceFlag{
			Name:    "hosts",
			Aliases: []string{"H"},
			Usage:   "hosts address to listen on",
			EnvVars: []string{"APACK_HOSTS"},
		},
		&cli.StringFlag{
			Name:  "data",
			Usage: "root directory for storage of apack data",
		},
		&cli.StringFlag{
			Name:  "tmp",
			Usage: "set the tmp directory",
		},
		&cli.IntFlag{
			Name:  "gc-percent",
			Usage: "set the garbage collection percent",
			Value: 30,
		},
		&cli.StringFlag{
			Name:  "memory-limit",
			Usage: "set the memory limit",
			Value: "4G",
		},
		&cli.BoolFlag{
			Name:  "secure",
			Usage: "set the secure flag",
		},
		&cli.StringSliceFlag{
			Name:  "mod",
			Value: cli.NewStringSlice(base.Namespace),
			Usage: "set the module to load, base API",
		},
		&cli.IntFlag{
			Name:  "concurrency",
			Usage: "the concurrents of artifact operations such as building, pushing, pulling, and save",
			Value: 1,
		},
		&cli.BoolFlag{
			Name:    "no-proxy",
			Aliases: []string{"np"},
			Usage:   "Disable proxy",
		},
		&cli.IntFlag{
			Name:  "compress",
			Usage: "the compression algorithm for the layer, where 0 represents the use of Gzip, 1 represents gzip-fastest, 2 represents zstd, and 3 means no compression",
			Value: 0,
		},
	},
	Action: func(ctx *cli.Context) error {
		cntxt := &dm.Context{
			PidFileName: "daemon.pid",
			PidFilePerm: 0o666,
			LogFileName: "daemon.log",
			LogFilePerm: 0o666,
			WorkDir:     "./",
			Umask:       027,
			Args:        ctx.Args().Slice(),
		}

		d, err := cntxt.Reborn()
		if err != nil {
			log.Logger.Fatal("Unable to run: ", err)
		}
		if d != nil {
			return nil
		}
		defer cntxt.Release()

		return program.Main(ctx, New, "ApackDaemon")
	},
}

type daemon struct {
	config *config.BaseConfig
	server *api.Server
}

func New(opts *options.Options) (program.App, error) {
	log.Logger.Debugf("Initializing Apack Daemon with opts: %+v", opts)
	d := &daemon{config: config.NewBase(opts)}
	log.Logger.Debugf(
		"Initialized Apack Daemon with config: %+v, opts: %+v", d.config, d.config.Opts)

	if err := os.MkdirAll(opts.Tmpdir, 0755); err != nil {
		return nil, fmt.Errorf(
			"failed to create temporary directory: %w", err)
	}

	if err := os.MkdirAll(opts.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	return d, d.config.Validate()
}

func (d *daemon) Run() error {
	log.Logger.Debugf(
		"Starting Apack Daemon with config: %+v, opts: %+v", d.config, d.config.Opts)
	ac := config.NewAPIConfig(d.config.Opts)
	ac.Base = d.config
	if err := ac.Validate(); err != nil {
		return err
	}
	server := api.New(ac)
	if err := server.Init(); err != nil {
		return err
	}
	d.server = server
	return server.Serve()
}

func (d *daemon) Stop() error {
	if d.server != nil {
		d.server.Done()
	}
	return nil
}

func (d *daemon) Self() string {
	return reflect.TypeOf(d).Elem().Name()
}

func (d *daemon) Ready() bool {
	return d.server.Check() == nil
}
