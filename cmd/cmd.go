package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/model-ci/apack/cmd/build"
	"github.com/model-ci/apack/cmd/daemon"
	"github.com/model-ci/apack/cmd/diff"
	"github.com/model-ci/apack/cmd/export"
	"github.com/model-ci/apack/cmd/gen"
	"github.com/model-ci/apack/cmd/importer"
	"github.com/model-ci/apack/cmd/info"
	"github.com/model-ci/apack/cmd/inspect"
	"github.com/model-ci/apack/cmd/kill"
	"github.com/model-ci/apack/cmd/list"
	"github.com/model-ci/apack/cmd/login"
	"github.com/model-ci/apack/cmd/logout"
	"github.com/model-ci/apack/cmd/ps"
	"github.com/model-ci/apack/cmd/pull"
	"github.com/model-ci/apack/cmd/push"
	"github.com/model-ci/apack/cmd/remove"
	"github.com/model-ci/apack/cmd/run"
	"github.com/model-ci/apack/cmd/tag"
	"github.com/model-ci/apack/cmd/version"
	"github.com/model-ci/apack/internal/consts"
	"github.com/model-ci/apack/internal/log"
	"github.com/urfave/cli/v2"
)

func rootConfigPath() string {
	var homeDir string

	if runtime.GOOS == "windows" {
		homeDir = os.Getenv("USERPROFILE")
	} else {
		homeDir = os.Getenv("HOME")
	}

	if homeDir == "" {
		homeDir, _ = os.Getwd()
	}

	return filepath.Join(homeDir, ".apack")
}

func Execute(name, usage, ver, commit string) {
	app := cli.NewApp()
	app.Name = name
	app.Usage = usage

	v := []string{ver}

	if commit != "" {
		v = append(v, "commit: "+commit)
	}
	v = append(v, "go: "+consts.GetGoVersion())
	v = append(v, "build: "+consts.GetBuildTime())
	app.Version = strings.Join(v, "\n")

	root := rootConfigPath()
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "host",
			Aliases: []string{"H"},
			Usage:   "apack daemon address",
			EnvVars: []string{"APACK_HOST"},
		},
		&cli.BoolFlag{
			Name:    "auth",
			Aliases: []string{"a"},
			Usage:   "Enable basic authentication",
			EnvVars: []string{"AUTH_ENABLED"},
		},
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "Basic auth username",
			EnvVars: []string{"USERNAME"},
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"pw"},
			Usage:   "Basic auth password",
			EnvVars: []string{"PASSWORD"},
		},
		&cli.StringFlag{
			Name:    "cert",
			Aliases: []string{"c"},
			Usage:   "TLS certificate file path",
			EnvVars: []string{"CERT_FILE"},
		},
		&cli.StringFlag{
			Name:    "key",
			Aliases: []string{"k"},
			Usage:   "TLS private key file path",
			EnvVars: []string{"KEY_FILE"},
		},
		&cli.StringFlag{
			Name:  "log",
			Usage: "set the log file to write apack logs to (default is '/dev/stderr')",
		},
		&cli.StringFlag{
			Name:  "log-level",
			Value: "debug",
			Usage: "set the log level ('DEBUG/debug', 'INFO/info', 'WARN/warn', 'ERROR/error', 'FATAL/fatal')",
		},
		&cli.StringFlag{
			Name:  "root",
			Value: root,
			Usage: "root directory for storage of apack config",
		},
	}

	app.Commands = []*cli.Command{
		gen.Command,
		login.Command,
		logout.Command,
		pull.Command,
		push.Command,
		build.Command,
		export.Command,
		list.Command,
		run.Command,
		ps.Command,
		kill.Command,
		remove.Command,
		inspect.Command,
		diff.Command,
		info.Command,
		daemon.Command,
		tag.Command,
		importer.Command,
		version.Command,
	}

	app.Before = func(ctx *cli.Context) error {
		if !ctx.IsSet("root") {
			if err := os.MkdirAll(root, 0o700); err != nil {
				_, err = fmt.Fprintln(os.Stderr, "the path in root must be writable by the user")
				return err
			}
			if err := os.Chmod(root, os.FileMode(0o700)|os.ModeSticky); err != nil {
				_, err = fmt.Fprintln(os.Stderr, "you should check permission of the path")
				return err
			}
		}
		log.Init(ctx.String("log"), ctx.String("log-level"))
		os.Setenv("TMPDIR", ctx.String("tmp"))

		//debug.SetGCPercent(ctx.Int("gc-percent"))
		//debug.SetMemoryLimit(int64(utils.MustParseSize(ctx.String("memory-limit"))))
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
	}
}
