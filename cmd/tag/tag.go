package tag

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
	Name:      "tag",
	Usage:     "Create a tag TARGET_ARTIFACTS that refers to SOURCE_ARTIFACTS",
	ArgsUsage: `SOURCE_ARTIFACTS[:TAG] TARGET_ARTIFACTS[:TAG]`,
	Description: `Create a tag TARGET_ARTIFACTS that refers to SOURCE_ARTIFACTS.

Examples:
  apack tag mymodel:latest mymodel:v1.0            # Tag latest version as v1.0
  apack tag mymodel:1.0 myregistry.com/mymodel:1.0 # Tag for different registry`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "host",
			Aliases: []string{"H"},
			Usage:   "Registry server address",
		},
	},
	Action: func(ctx *cli.Context) error {
		tag, err := NewTag(ctx)
		if err != nil {
			return err
		}
		return tag.Run()
	},
}

type Tag struct {
	ctx       context.Context
	operation string
	srcRef    registry.Reference
	dstRef    registry.Reference
	host      string
	client    *client.BaseClient
}

func NewTag(ctx *cli.Context) (*Tag, error) {
	tag := &Tag{ctx: context.Background(), operation: "tag"}
	tag.host = ctx.String("host")

	args := ctx.Args().Slice()
	if len(args) != 2 {
		return nil, fmt.Errorf("requires exactly 2 arguments: SOURCE_ARTIFACTS[:TAG] TARGET_ARTIFACTS[:TAG]")
	}

	srcRef, err := registry.ParseReference(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid source artifact reference %q: %w", args[0], err)
	}
	tag.srcRef = srcRef

	dstRef, err := registry.ParseReference(args[1])
	if err != nil {
		return nil, fmt.Errorf("invalid target artifact reference %q: %w", args[1], err)
	}
	tag.dstRef = dstRef

	return tag, tag.completeAndValidate()
}

func (t *Tag) completeAndValidate() error {
	var err error
	if t.client == nil {
		t.client, err = client.NewDefaultCLI(t.host)
		if err != nil {
			return err
		}
	}

	if t.srcRef.String() == t.dstRef.String() {
		return fmt.Errorf("source and target references cannot be identical")
	}

	if t.srcRef.Repository == "" {
		return fmt.Errorf("source repository name cannot be empty")
	}
	if t.dstRef.Repository == "" {
		return fmt.Errorf("target repository name cannot be empty")
	}

	return nil
}

func (t *Tag) Run() error {
	req := types.Request{
		Args: types.Args{
			TargetRef: t.dstRef,
		},
		ReferenceStr: t.srcRef.String(),
		Reference:    t.srcRef,
	}

	resp, err := t.client.Post(t.ctx, base.API("/v1/tag"), req)
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
		fmt.Printf("Successfully tagged %s as %s\n", t.srcRef.String(), t.dstRef.String())
		return nil
	default:
		return fmt.Errorf("Failed to tag model: %s", res.Message)
	}
}
