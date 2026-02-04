package ps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/runtime"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/pkg/client"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:      "ps",
	Usage:     "List running models",
	ArgsUsage: `[OPTIONS]`,
	Description: `List running models in the system.

Examples:
  apack ps                              # List running models
  apack ps -a                           # List all models (including stopped)
  apack ps --filter "status=running"    # List models with specific filter
  apack ps -q                           # Only show model IDs`,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "all",
			Aliases: []string{"a"},
			Usage:   "Show all models (default shows just running)",
		},
		&cli.StringFlag{
			Name:  "filter",
			Usage: "Filter output based on conditions provided",
		},
		&cli.StringFlag{
			Name:  "format",
			Value: "table",
			Usage: "Pretty-print models using a Go template",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Only show model IDs",
		},
	},
	Action: func(ctx *cli.Context) error {
		ps, err := NewPs(ctx)
		if err != nil {
			return err
		}
		return ps.Run()
	},
}

type Ps struct {
	ctx    context.Context
	client *client.BaseClient
	All    bool
	Filter string
	Format string
	Quiet  bool
	host   string
}

func NewPs(ctx *cli.Context) (*Ps, error) {
	ps := &Ps{ctx: context.Background()}

	ps.All = ctx.Bool("all")
	ps.Filter = ctx.String("filter")
	ps.Format = ctx.String("format")
	ps.Quiet = ctx.Bool("quiet")
	ps.host = ctx.String("host")

	return ps, ps.completeAndValidate()
}

func (p *Ps) completeAndValidate() error {
	var err error
	p.client, err = client.NewDefaultCLI(p.host)
	if err != nil {
		return err
	}
	return nil
}

func (p *Ps) Run() error {
	req := types.Request{}

	resp, err := p.client.Post(p.ctx, base.API("/v1/ps"), req)
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
		states := &runtime.States{}
		err = states.UnmarshalJSON(res.Message)
		if err != nil {
			return err
		}
		return p.formatAndPrint(os.Stdout, *states)
	default:
		return fmt.Errorf("Not found model runtimes: %s", res.Message)
	}
}

func (p *Ps) formatAndPrint(w io.Writer, runtime runtime.States) error {
	if runtime.Count == 0 {
		return fmt.Errorf("No runtime state found")
	}

	switch p.Format {
	case "table":
		printSummary(w, runtime)
	case "json":
		jsonBytes, err := json.MarshalIndent(runtime, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(jsonBytes))
	default:
		return fmt.Errorf("unsupported format %s", p.Format)
	}
	return nil
}
