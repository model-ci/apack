package consts

import "runtime"

var (
	version   string
	gitCommit string
	buildTime string

	goVersion = runtime.Version()
)

func GetVersion() string {
	if version == "" {
		return "unknown"
	}
	return version
}

func GetGoVersion() string {
	if goVersion == "" {
		return "unknown"
	}
	return goVersion
}

func GetGitCommit() string {
	if gitCommit == "" {
		return "none"
	}
	return gitCommit
}

func GetBuildTime() string {
	if buildTime == "" {
		return "no"
	}
	return buildTime
}
