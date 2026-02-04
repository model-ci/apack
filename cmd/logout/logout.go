package logout

import (
	"context"
	"fmt"

	"github.com/model-ci/apack/internal/config"
	"github.com/model-ci/apack/internal/repo"
	"github.com/model-ci/apack/pkg/client"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:      "logout",
	Usage:     "Log out from a registry",
	ArgsUsage: `[SERVER]`,
	Description: `Log out from a registry.
If no server is specified, the default registry will be used.

Examples:
  apack logout                          # Logout from default registry
  apack logout localhost:5000           # Logout from localhost:5000
  apack logout registry.example.com     # Logout from registry.example.com`,
	Action: func(ctx *cli.Context) error {
		logout, err := NewLogout(ctx)
		if err != nil {
			return err
		}
		return logout.Run()
	},
}

type Logout struct {
	ctx       context.Context
	client    *client.BaseClient
	operation string
	Registry  string
}

func NewLogout(ctx *cli.Context) (*Logout, error) {
	logout := &Logout{ctx: context.Background(), operation: "logout"}
	registry := ctx.Args().First()
	if registry == "" {
		logout.Registry = "https://index.docker.io/v1/"
	} else {
		logout.Registry = registry
	}

	return logout, nil
}

func (l *Logout) Run() error {
	configPath := config.JsonPath("")
	store, err := repo.NewCredentialStore(configPath)
	if err != nil {
		return err
	}
	if err := repo.CredentialsLogout(l.ctx, store, l.Registry); err != nil {
		return err
	}

	fmt.Printf("Logout Succeeded\n")
	return nil
}
