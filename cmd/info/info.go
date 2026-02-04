package info

import (
	"context"
	"fmt"

	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/pkg/client"
	"github.com/urfave/cli/v2"
	"oras.land/oras-go/v2/registry"
)

var Command = &cli.Command{
	Name:      "info",
	Usage:     "Display system-wide information",
	ArgsUsage: ``,
	Description: `Display system-wide information about the Apack installation.
This includes kernel version, number of containers and images, etc.

Examples:
  apack push mymodel                      # Push mymodel:latest
  apack push mymodel:v1.0                 # Push mymodel with tag v1.0
  apack info mymodel:v1.0 --format json   # Show info in JSON format`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "format",
			Usage: "Format the output using the given Go template",
		},
	},
	Action: func(ctx *cli.Context) error {
		info, err := NewInfo(ctx)
		if err != nil {
			return err
		}
		return info.Run()
	},
}

type Info struct {
	ctx       context.Context
	client    *client.BaseClient
	Format    string
	tag       string
	reference registry.Reference
	host      string
}

func NewInfo(ctx *cli.Context) (*Info, error) {
	info := &Info{ctx: context.Background()}

	tag := ctx.Args().First()
	if tag == "" {
		info.tag = ""
	} else {
		info.tag = tag
	}

	info.Format = ctx.String("format")
	info.host = ctx.String("host")

	return info, info.completeAndValidate()
}

func (i *Info) completeAndValidate() error {
	var err error
	i.client, err = client.NewDefaultCLI(i.host)
	if err != nil {
		return err
	}
	if i.tag == "" {
		return fmt.Errorf(
			"image name is required, please specify an image name")
	} else {
		i.reference, err = registry.ParseReference(i.tag)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *Info) Run() error {

	req := types.Request{Reference: i.reference, ReferenceStr: i.tag}

	resp, err := i.client.Post(i.ctx, base.API("/v1/info"), req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	res := &types.Response{}
	if err := res.Decode(resp.Body); err != nil {
		return err
	}

	if res.Message == nil {
		return fmt.Errorf("not found message")
	}

	artifact := &spec.Artifact{}
	err = artifact.UnmarshalJSON(res.Message)
	if err != nil {
		return err
	}

	return printInfo(artifact)
}

func printInfo(info *spec.Artifact) error {
	b, err := info.MarshalToYAML()
	if err != nil {
		return err
	}
	fmt.Println(b)
	return nil
}
