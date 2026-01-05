package pull

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/config"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/internal/utils"
	"github.com/urfave/cli/v2"
	"oras.land/oras-go/v2/registry"
)

var Command = &cli.Command{
	Name:      "pull",
	Usage:     "Pull an image from a registry",
	ArgsUsage: `IMAGE[:TAG|@DIGEST]`,
	Description: `Pull an image from a registry.
If no tag or digest is specified, latest will be used.

Examples:
  apack pull ubuntu                     # Pull ubuntu:latest
  apack pull ubuntu:20.04               # Pull ubuntu with tag 20.04
  apack pull registry.example.com/app   # Pull from custom registry`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "registry",
			Aliases: []string{"r"},
			Usage:   "Registry server address",
		},
		&cli.BoolFlag{
			Name:  "all-tags",
			Usage: "Download all tagged images in the repository",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Suppress verbose output",
		},
	},
	Action: func(ctx *cli.Context) error {
		pull, err := NewPull(ctx)
		if err != nil {
			return err
		}
		return pull.Run()
	},
}

type Pull struct {
	ctx          context.Context
	client       *utils.Client
	operation    string
	ReferenceStr string
	Reference    registry.Reference
	AllTags      bool
	Quiet        bool
	rootConfig   string
	host         string
}

func NewPull(ctx *cli.Context) (*Pull, error) {
	pull := &Pull{ctx: context.Background(), operation: "pull"}

	pull.ReferenceStr = ctx.Args().First()
	pull.AllTags = ctx.Bool("all-tags")
	pull.Quiet = ctx.Bool("quiet")
	pull.rootConfig = ctx.String("root")
	pull.host = ctx.String("host")

	return pull, pull.completeAndValidate()
}

func (p *Pull) completeAndValidate() error {
	var err error
	p.client, err = utils.NewDefaultClient(p.host)
	if err != nil {
		return err
	}

	if p.ReferenceStr == "" {
		return fmt.Errorf("image reference is required")
	}

	ref, err := registry.ParseReference(p.ReferenceStr)
	if err != nil {
		return fmt.Errorf("invalid image reference %q: %w", p.ReferenceStr, err)
	}
	p.Reference = ref

	if ref.Reference == "" && !p.AllTags {
		refWithLatest := p.ReferenceStr + ":latest"
		ref, err = registry.ParseReference(refWithLatest)
		if err != nil {
			return fmt.Errorf("failed to parse reference with latest tag: %w", err)
		}
		p.Reference = ref
		p.ReferenceStr = refWithLatest

		if !p.Quiet {
			fmt.Printf("Using default tag: latest\n")
		}
	}

	if p.AllTags && ref.Reference != "" {
		return fmt.Errorf("cannot use --all-tags with a specific tag or digest")
	}

	if p.client == nil {
		return fmt.Errorf("failed to initialize registry client")
	}

	if err := p.validateImageReference(ref); err != nil {
		return err
	}

	return nil
}

func (p *Pull) validateImageReference(ref registry.Reference) error {
	if ref.Repository == "" {
		return fmt.Errorf("repository name cannot be empty")
	}

	if ref.Reference != "" {
		if strings.HasPrefix(ref.Reference, "sha256:") {
			if len(ref.Reference) != 71 {
				return fmt.Errorf("invalid digest format: %s", ref.Reference)
			}
		} else {
			if err := p.validateTagFormat(ref.Reference); err != nil {
				return fmt.Errorf("invalid tag format: %w", err)
			}
		}
	}

	return nil
}

func (p *Pull) validateTagFormat(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}

	if len(tag) > 128 {
		return fmt.Errorf("tag too long (max 128 characters)")
	}

	if strings.Contains(tag, " ") {
		return fmt.Errorf("tag cannot contain spaces")
	}

	return nil
}

func (p *Pull) Run() error {
	req := types.Request{
		Reference: p.Reference,
	}

	configPath := config.JsonPath(p.rootConfig)
	if p.client.IsRemote() {
		if data, err := os.ReadFile(configPath); err == nil {
			req.ConfigJSON = data
		}
	}

	resp, err := p.client.Post(p.ctx, base.API("/v1/pull"), req)
	if err != nil {
		return err
	}

	res := &types.Response{}
	if err := res.Decode(resp.Body); err != nil {
		return err
	}

	fmt.Printf("Pulling %s\n", req.Reference.String())
	switch resp.StatusCode {
	case http.StatusOK:
		if !p.Quiet {
			err = p.client.WatchProgress(p.ctx, res.TaskID, p.operation)
			if err != nil {
				return err
			}
			return nil
		}
	default:
		return fmt.Errorf("Failed to pull model image: %s", res.Message)
	}

	return nil
}
