package main

import (
	"github.com/model-ci/apack/cmd"
	"github.com/model-ci/apack/internal/consts"
	"github.com/model-ci/apack/internal/program"
)

const (
	usage = `Artifacts packaging and versioning tool for AI models, datasets, code, and configuration in accordance with the OCI specification.`
)

func main() {
	cmd.Execute(program.Name, usage, consts.GetVersion(), consts.GetGitCommit())
}
