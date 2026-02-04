package login

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/model-ci/apack/internal/config"
	"github.com/model-ci/apack/internal/repo"
	"github.com/model-ci/apack/pkg/client"
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

var Command = &cli.Command{
	Name:      "login",
	Usage:     "Log in to a registry",
	ArgsUsage: `[SERVER]`,
	Description: `Log in to a registry.
If no server is specified, the default registry will be used.

Examples:
  apack login                          # Login to default registry
  apack login localhost:5000           # Login to localhost:5000
  apack login registry.example.com     # Login to registry.example.com`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "Username",
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p"},
			Usage:   "Password",
		},
		&cli.BoolFlag{
			Name:  "password-stdin",
			Usage: "Take the password from stdin",
		},
	},
	Action: func(ctx *cli.Context) error {
		login, err := NewLogin(ctx)
		if err != nil {
			return err
		}
		return login.Run()
	},
}

type Login struct {
	client    *client.BaseClient
	operation string
	ctx       context.Context
	Registry  string
	Username  string
	Password  string
}

func NewLogin(ctx *cli.Context) (*Login, error) {
	login := &Login{ctx: context.Background(), operation: "login"}

	registry := ctx.Args().First()
	if registry == "" {
		login.Registry = "https://index.docker.io/v1/"
	} else {
		login.Registry = registry
	}

	if username := ctx.String("username"); username != "" {
		login.Username = username
	} else {
		fmt.Print("Username: ")
		if _, err := fmt.Scanln(&login.Username); err != nil {
			return nil, fmt.Errorf("failed to read username: %v", err)
		}
	}

	if ctx.Bool("password-stdin") {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			login.Password = scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read password from stdin: %v", err)
		}
	} else if password := ctx.String("password"); password != "" {
		login.Password = password
	} else {
		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return nil, fmt.Errorf("failed to read password: %v", err)
		}
		login.Password = string(passwordBytes)
		fmt.Println()
	}

	var err error
	login.client, err = client.NewDefaultCLI(ctx.String("host"))
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	return login, nil
}

func (l *Login) Run() error {
	if l.Username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if l.Password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	if err := l.Execute(); err != nil {
		return err
	}

	fmt.Printf("Login Succeeded\n")
	return nil
}

func (l *Login) Execute() error {
	configPath := config.JsonPath("")
	store, err := repo.NewCredentialStore(configPath)
	if err != nil {
		return err
	}
	opts := &distribution.Options{
		PlainHTTP: true,
	}
	registry, err := repo.NewRegistry(l.Registry, opts)
	if err != nil {
		return fmt.Errorf("could not resolve registry %s: %w", l.Registry, err)
	}
	cred := repo.Cred{
		Username: l.Username,
		Password: l.Password,
	}
	if err := repo.CredentialsLogin(l.ctx, store, registry, cred); err != nil {
		return err
	}
	return nil
}
