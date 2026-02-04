package run

import (
	"context"
	"fmt"
	"net/http"

	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/pkg/client"
	"github.com/urfave/cli/v2"
	"oras.land/oras-go/v2/registry"
)

var Command = &cli.Command{
	Name:      "run",
	Usage:     "Run a command in a new model image",
	ArgsUsage: `IMAGE [COMMAND] [ARG...]`,
	Description: `Run a command in a new model image.

Examples:
  apack run my-model                    # Run latest model image
  apack run my-model:v1.0               # Run model image with tag
  apack run -p 1985 my-model:v1.0       # Run model image with tag and specified port
`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "name",
			Usage: "Assign a name to the runtime model",
		},
		&cli.StringSliceFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Usage:   "Publish a model runtime's port(s) to the host",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Suppress verbose output",
		},
	},
	Action: func(ctx *cli.Context) error {
		r, err := NewRun(ctx)
		if err != nil {
			return err
		}
		return r.Run()
	},
}

type Run struct {
	ctx       context.Context
	client    *client.BaseClient
	Reference registry.Reference
	Image     string
	Command   []string
	Name      string
	Ports     []string
	host      string
	Quiet     bool
	operation string
}

func NewRun(ctx *cli.Context) (*Run, error) {
	run := &Run{ctx: context.Background(), operation: "run"}

	args := ctx.Args().Slice()
	if len(args) > 0 {
		run.Image = args[0]
		if len(args) > 1 {
			run.Command = args[1:]
		}
	}

	run.Name = ctx.String("name")
	run.Ports = ctx.StringSlice("port")
	run.host = ctx.String("host")

	return run, run.completeAndValidate()
}

func (r *Run) completeAndValidate() error {
	var err error
	if r.client == nil {
		r.client, err = client.NewDefaultCLI(r.host)
		if err != nil {
			return err
		}
	}

	if r.Image == "" {
		return fmt.Errorf(
			"image name is required, please specify an image name")
	} else {
		r.Reference, err = registry.ParseReference(r.Image)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Run) Run() error {
	req := types.Request{
		Reference:    r.Reference,
		ReferenceStr: r.Reference.String(),
	}

	resp, err := r.client.Post(r.ctx, base.API("/v1/run"), req)
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
		if !r.Quiet {
			fmt.Printf("Running %s\n", r.Image)
		}
	default:
		return fmt.Errorf("Failed to run model: %s", res.Message)
	}
	return nil
}
