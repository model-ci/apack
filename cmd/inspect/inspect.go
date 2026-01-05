package inspect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/internal/utils"
	"github.com/urfave/cli/v2"
	"oras.land/oras-go/v2/registry"
)

var Command = &cli.Command{
	Name:      "inspect",
	Usage:     "Return low-level information on Apack objects",
	ArgsUsage: `NAME|ID [NAME|ID...]`,
	Description: `Return low-level information on Apack objects (images, containers, etc.).

Examples:
  apack inspect mymodel:latest          # Inspect model
`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "format",
			Aliases: []string{"f"},
			Usage:   "Format the output using the given Go template",
		},
	},
	Action: func(ctx *cli.Context) error {
		inspect, err := NewInspect(ctx)
		if err != nil {
			return err
		}
		return inspect.Run()
	},
}

type Inspect struct {
	ctx       context.Context
	client    *utils.Client
	Format    string
	Tag       string
	reference registry.Reference
	host      string
}

func NewInspect(ctx *cli.Context) (*Inspect, error) {
	inspect := &Inspect{ctx: context.Background()}

	tag := ctx.Args().First()
	if tag == "" {
		inspect.Tag = ""
	} else {
		inspect.Tag = tag
	}

	inspect.Format = ctx.String("format")
	inspect.host = ctx.String("host")

	return inspect, inspect.completeAndValidate()
}

func (i *Inspect) completeAndValidate() error {
	var err error
	i.client, err = utils.NewDefaultClient(i.host)
	if err != nil {
		return err
	}
	if i.Tag == "" {
		return fmt.Errorf(
			"image name is required, please specify an image name")
	} else {
		i.reference, err = registry.ParseReference(i.Tag)
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *Inspect) Run() error {
	inspect, err := Insepection(i.ctx, i.client, i.reference)
	if err != nil {
		return err
	}
	prettyJson, _ := json.MarshalIndent(inspect, "", "  ")
	fmt.Println(string(prettyJson))
	return nil
}

func Insepection(ctx context.Context, cli *utils.Client, ref registry.Reference) (*types.Inspect, error) {
	req := types.Request{Reference: ref, ReferenceStr: ref.String()}

	resp, err := cli.Post(ctx, base.API("/v1/inspect"), req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	res := &types.Response{}
	if err := res.Decode(resp.Body); err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		inspect := &types.Inspect{}
		err = inspect.UnmarshalJSON(res.Message)
		if err != nil {
			return nil, err
		}
		return inspect, nil
	default:
		return nil, fmt.Errorf("Inspect failure: %s", res.Message)
	}
}
