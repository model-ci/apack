package list

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	gotemplate "text/template"

	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/internal/utils"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/urfave/cli/v2"
	"oras.land/oras-go/v2/registry"
)

var Command = &cli.Command{
	Name:      "list",
	Aliases:   []string{"ls"},
	Usage:     "List model images",
	ArgsUsage: `[OPTIONS]`,
	Description: `List model images in the system.

Examples:
  apack list   # List all images
  apack ls -a  # List all images including intermediate
  `,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "all",
			Aliases: []string{"a"},
			Usage:   "Show all images (default hides intermediate images)",
		},
		&cli.StringFlag{
			Name:  "filter",
			Usage: "Filter output based on conditions provided",
		},
		&cli.StringFlag{
			Name:  "format",
			Value: "table",
			Usage: "Pretty-print images using a Go template",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Only show image IDs",
		},
	},
	Action: func(ctx *cli.Context) error {
		list, err := NewList(ctx)
		if err != nil {
			return err
		}
		return list.Run()
	},
}

type options struct {
	distribution.Options
	configHome string
	remoteRef  *registry.Reference
	format     string
	template   string
}

type List struct {
	distribution.Options
	ctx        context.Context
	client     *utils.Client
	configHome string
	remoteRef  *registry.Reference
	format     string
	template   string
	all        bool
	filter     string
	quiet      bool
	operation  string
	host       string
}

func NewList(ctx *cli.Context) (*List, error) {
	list := &List{ctx: context.Background(), operation: "list"}

	list.all = ctx.Bool("all")
	list.filter = ctx.String("filter")
	list.format = ctx.String("format")
	list.quiet = ctx.Bool("quiet")
	list.host = ctx.String("host")

	return list, list.completeAndValidate()
}

func (l *List) completeAndValidate() error {
	var err error
	l.client, err = utils.NewDefaultClient(l.host)
	if err != nil {
		return err
	}
	return nil
}

func (l *List) Run() error {
	req := types.Request{}

	resp, err := l.client.Post(l.ctx, base.API("/v1/list"), req)
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
		artifacts := &types.Artifacts{}
		err = artifacts.UnmarshalJSON(res.Message)
		if err != nil {
			return err
		}
		return l.formatAndPrint(os.Stdout, *artifacts)
	default:
		return fmt.Errorf("Status failure: %s", res.Message)
	}
}

func (l *List) formatAndPrint(w io.Writer, artifacts types.Artifacts) error {
	if artifacts.Count == 0 {
		return fmt.Errorf("No artifact found")
	}

	switch l.format {
	case "table":
		printSummary(w, artifacts)
	case "json":
		jsonBytes, err := json.MarshalIndent(artifacts, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(w, string(jsonBytes))
	case "template":
		tpl, err := gotemplate.New("list").Parse(l.template)
		if err != nil {
			return err
		}
		for _, item := range artifacts.Items {
			if err := tpl.Execute(w, item); err != nil {
				return err
			}
			if !strings.HasSuffix(l.template, "\n") {
				fmt.Fprintln(w)
			}
		}
	default:
		return fmt.Errorf("unsupported format %s", l.format)
	}
	return nil
}
