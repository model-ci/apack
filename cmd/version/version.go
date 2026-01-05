package version

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/model-ci/apack/internal/consts"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "version",
	Usage: "Show version information",
	Action: func(ctx *cli.Context) error {
		return showVersion(ctx)
	},
}

func showVersion(ctx *cli.Context) error {
	version := consts.GetVersion()
	gitCommit := consts.GetGitCommit()
	goVersion := consts.GetGoVersion()
	buildTime := consts.GetBuildTime()

	var info []string

	if version != "unknown" {
		info = append(info, fmt.Sprintf("Version: %s", version))
	}

	if gitCommit != "none" {
		info = append(info, fmt.Sprintf("Git Commit: %s", gitCommit))
	}

	if goVersion != "unknown" {
		info = append(info, fmt.Sprintf("Go Version: %s", goVersion))
	}

	if buildTime != "no" {
		info = append(info, fmt.Sprintf("Build Time: %s", buildTime))
	}

	// Add platform information
	info = append(info, fmt.Sprintf("OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))

	fmt.Println(strings.Join(info, "\n"))
	return nil
}
