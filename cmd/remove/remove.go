package remove

import (
	"context"
	"fmt"
	"net/http"

	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/pkg/client"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:      "remove",
	Aliases:   []string{"rm"},
	Usage:     "Remove one or more artifacts",
	ArgsUsage: `artifact [artifact...]`,
	Description: `Remove one or more artifacts from local storage.
Use -f to force removal of artifacts that are being used by containers.

Examples:
  apack remove mymodel       # Remove mymodel:latest
  apack remove mymodel:v1.0  # Remove mymodel with tag v1.0
  apack rm -f mymodel        # Force remove artifact`,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "Force removal of the artifact",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Suppress verbose output",
		},
	},
	Action: func(ctx *cli.Context) error {
		r, err := NewRemove(ctx)
		if err != nil {
			return err
		}
		return r.Run()
	},
}

type Remove struct {
	ctx      context.Context
	artifact string
	Force    bool
	Quiet    bool
	host     string
	client   *client.BaseClient
}

func NewRemove(ctx *cli.Context) (*Remove, error) {
	remove := &Remove{ctx: context.Background()}

	artifacts := ctx.Args().Slice()
	remove.artifact = artifacts[0]

	remove.Force = ctx.Bool("force")
	remove.Quiet = ctx.Bool("quiet")
	remove.host = ctx.String("host")

	return remove, remove.completeAndValidate()
}

func (r *Remove) completeAndValidate() error {
	var err error
	r.client, err = client.NewDefaultCLI(r.host)
	if err != nil {
		return err
	}
	if r.artifact == "" {
		return fmt.Errorf("at least one artifact name is required")
	}
	return nil
}

func (r *Remove) Run() error {
	req := types.Request{
		ReferenceStr: r.artifact,
	}

	resp, err := r.client.Post(r.ctx, base.API("/v1/remove"), req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	res := &types.Response{}
	if err := res.Decode(resp.Body); err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		fmt.Printf("Kill Succeeded: %s\n", r.artifact)
		return nil

	default:
		return fmt.Errorf("Failed to kill: %s", res.Message)
	}
}
