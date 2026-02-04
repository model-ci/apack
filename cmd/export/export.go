package export

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/pkg/client"
	"github.com/urfave/cli/v2"
	"oras.land/oras-go/v2/registry"
)

var Command = &cli.Command{
	Name:      "export",
	Usage:     "export an image to files or directories",
	ArgsUsage: `SOURCE [DESTINATION]`,
	Description: `export an image to files or directories.

Examples:
  apack export mymodel:latest              # export to current directory
  apack export mymodel:latest -o ./output  # export to specific directory
  apack export --strip-components=1 app    # Strip path components`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   "./",
			Usage:   "Output directory",
		},
		&cli.BoolFlag{
			Name:  "overwrite",
			Usage: "Overwrite existing files",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Suppress verbose output",
		},
	},
	Action: func(ctx *cli.Context) error {
		e, err := Newexport(ctx)
		if err != nil {
			return err
		}
		return e.Run()
	},
}

type export struct {
	ctx       context.Context
	client    *client.BaseClient
	Reference registry.Reference
	host      string
	operation string
	Source    string
	Output    string
	Overwrite bool
	Quiet     bool
}

func Newexport(ctx *cli.Context) (*export, error) {
	export := &export{ctx: context.Background(), operation: "export"}

	source := ctx.Args().First()
	export.Source = source
	export.host = ctx.String("host")
	export.Overwrite = ctx.Bool("overwrite")
	export.Quiet = ctx.Bool("quiet")
	export.Output = ctx.String("output")
	return export, export.completeAndValidate()
}

func (e *export) completeAndValidate() error {
	var err error

	if e.client == nil {
		e.client, err = client.NewDefaultCLI(e.host)
		if err != nil {
			return err
		}
	}

	if e.Source == "" {
		return fmt.Errorf("source image name cannot be empty")
	} else {
		e.Reference, err = registry.ParseReference(e.Source)
		if err != nil {
			return err
		}
	}

	if e.Output != "" {
		e.Output, err = filepath.Abs(e.Output)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *export) Run() error {
	args := types.Args{
		Output:    e.Output,
		Overwrite: e.Overwrite,
	}

	req := &types.Request{
		Reference:    e.Reference,
		ReferenceStr: e.Reference.String(),
		Args:         args,
	}

	resp, err := e.client.Post(e.ctx, base.API("/v1/export"), req)
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
		if !e.Quiet {
			err = e.client.WatchProgress(e.ctx, res.TaskID, e.operation)
			if err != nil {
				return err
			}
			fmt.Printf("Using : %s, export finished\n", e.Source)
		}

	default:
		return fmt.Errorf("Failed to export model: %s", res.Message)
	}

	return nil
}
