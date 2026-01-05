package kill

import (
	"context"
	"fmt"
	"net/http"

	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/internal/utils"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:      "kill",
	Usage:     "Kill one or more running runtimes",
	ArgsUsage: `runtime [runtime...]`,
	Description: `Kill one or more running runtimes using SIGKILL or a specified signal.

Examples:
  apack kill runtime-id                # Kill runtime with SIGKILL
  apack kill --signal=SIGTERM runtime  # Kill with SIGTERM`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "signal",
			Aliases: []string{"s"},
			Value:   "KILL",
			Usage:   "Signal to send to the runtime",
		},
	},
	Action: func(ctx *cli.Context) error {
		k, err := NewKill(ctx)
		if err != nil {
			return err
		}
		return k.Run()
	},
}

type Kill struct {
	ctx    context.Context
	client *utils.Client
	id     string
	Signal string
	host   string
}

func NewKill(ctx *cli.Context) (*Kill, error) {
	kill := &Kill{ctx: context.Background()}

	id := ctx.Args().Slice()
	kill.id = id[0]
	kill.Signal = ctx.String("signal")
	kill.host = ctx.String("host")

	return kill, kill.completeAndValidate()
}

func (k *Kill) completeAndValidate() error {
	var err error
	k.client, err = utils.NewDefaultClient(k.host)
	if err != nil {
		return err
	}
	if k.id == "" {
		return fmt.Errorf("at least one runtime name is required")
	}
	return nil
}

func (k *Kill) Run() error {
	req := types.Request{
		ID: k.id,
	}

	resp, err := k.client.Post(k.ctx, base.API("/v1/kill"), req)
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
		fmt.Printf("Kill Succeeded: %s\n", k.id)
		return nil

	default:
		return fmt.Errorf("Failed to kill: %s", res.Message)
	}
}
