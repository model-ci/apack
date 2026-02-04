package push

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
	Name:      "push",
	Usage:     "Push an image to a registry",
	ArgsUsage: `IMAGE[:TAG]`,
	Description: `Push an image to a registry.
If no tag is specified, latest will be used.

Examples:
  apack push mymodel                      # Push mymodel:latest
  apack push mymodel:v1.0                 # Push mymodel with tag v1.0
  apack push registry.example.com/model   # Push to custom registry`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "registry",
			Aliases: []string{"r"},
			Usage:   "Registry server address",
		},
		&cli.BoolFlag{
			Name:  "all-tags",
			Usage: "Push all tagged images in the repository",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Suppress verbose output",
		},
		&cli.BoolFlag{
			Name:  "disable-content-trust",
			Usage: "Skip image signing",
		},
	},
	Action: func(ctx *cli.Context) error {
		p, err := NewPush(ctx)
		if err != nil {
			return err
		}
		return p.Run()
	},
}

type Push struct {
	ctx                 context.Context
	client              *client.BaseClient
	Tag                 string
	Reference           registry.Reference
	Registry            string
	AllTags             bool
	Quiet               bool
	DisableContentTrust bool
	host                string
	operation           string
}

func NewPush(ctx *cli.Context) (*Push, error) {
	push := &Push{ctx: context.Background(), operation: "push"}

	tag := ctx.Args().First()
	if tag == "" {
		push.Tag = ""
	} else {
		push.Tag = tag
	}

	registry := ctx.String("registry")
	if registry == "" {
		push.Registry = "https://index.docker.io/v1/"
	} else {
		push.Registry = registry
	}

	push.AllTags = ctx.Bool("all-tags")
	push.Quiet = ctx.Bool("quiet")
	push.DisableContentTrust = ctx.Bool("disable-content-trust")
	push.host = ctx.String("host")

	return push, push.completeAndValidate()
}

func (p *Push) completeAndValidate() error {
	var err error
	if p.client == nil {
		p.client, err = client.NewDefaultCLI(p.host)
		if err != nil {
			return err
		}
	}

	if p.Registry != "" {
		p.Reference.Registry = p.Registry
	}

	if p.Tag == "" {
		return fmt.Errorf(
			"image name is required, please specify an image name")
	} else {
		p.Reference, err = registry.ParseReference(p.Tag)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Push) Run() error {
	req := types.Request{
		Reference: p.Reference,
	}

	resp, err := p.client.Post(p.ctx, base.API("/v1/push"), req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	res := &types.Response{}
	if err := res.Decode(resp.Body); err != nil {
		return err
	}

	fmt.Printf("Pushing %s\n", req.Reference.String())
	switch resp.StatusCode {
	case http.StatusOK:
		if !p.Quiet {
			err = p.client.WatchProgress(p.ctx, res.TaskID, p.operation)
			if err != nil {
				return err
			}
		}

	default:
		return fmt.Errorf("Failed to push model image: %s", res.Message)
	}

	return nil
}
