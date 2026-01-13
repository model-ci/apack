package importer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/internal/utils"
	"github.com/model-ci/apack/pkg/tools/ollama"
	"github.com/urfave/cli/v2"
	"oras.land/oras-go/v2/registry"
)

var Command = &cli.Command{
	Name:      "import",
	Usage:     "Import an artifact from 3rd party registry",
	ArgsUsage: `ARTIFACT[:TAG|@DIGEST]`,
	Description: `Import an artifact from cloud hub.
If no tag or digest is specified, latest will be used.

Examples:
  apack import qwen3:1.7b --tools=ollama                  # Import qwen3:1.7b from ollama
  apack import llama3.2-1b:1B-instruct-q4_0 --tools=jozu  # Import llama3.2-1b:1B-instruct-q4_0 from jozu`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "username",
			Value:   "apack",
			Aliases: []string{"u"},
			Usage:   "User name",
			EnvVars: []string{"APACK_IMPORT_USERNAME"},
		},
		&cli.StringFlag{
			Name:    "password",
			Value:   "ai",
			Aliases: []string{"p"},
			Usage:   "User password",
			EnvVars: []string{"APACK_IMPORT_PASSWORD"},
		},
		&cli.StringFlag{
			Name:    "tools",
			Aliases: []string{"t"},
			Usage:   "Use tools from the specified toolchain, support kitops/jozu, ollama",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Suppress verbose output",
		},
	},
	Action: func(ctx *cli.Context) error {
		importer, err := NewImporter(ctx)
		if err != nil {
			return err
		}
		return importer.Run()
	},
}

type Importer struct {
	ctx context.Context

	operation    string
	referenceStr string
	reference    registry.Reference
	quiet        bool
	host         string
	username     string
	password     string
	tools        string
	client       *utils.Client
}

func NewImporter(ctx *cli.Context) (*Importer, error) {
	importer := &Importer{ctx: context.Background(), operation: "import"}

	importer.referenceStr = ctx.Args().First()
	importer.quiet = ctx.Bool("quiet")
	importer.host = ctx.String("host")
	importer.username = ctx.String("username")
	importer.password = ctx.String("password")
	importer.tools = ctx.String("tools")

	return importer, importer.completeAndValidate()
}

func (i *Importer) completeAndValidate() error {
	var err error
	i.client, err = utils.NewDefaultClient(i.host)
	if err != nil {
		return err
	}

	if i.referenceStr == "" {
		return fmt.Errorf("artifact reference is required")
	}

	i.reference = ollama.MakeReference(i.referenceStr)
	if err := i.reference.Validate(); err != nil {
		return err
	}

	if i.client == nil {
		return fmt.Errorf("failed to initialize registry client")
	}

	if i.tools == "" {
		return fmt.Errorf("tools is required")
	}

	return nil
}

func (i *Importer) Run() error {
	req := types.Request{
		Import: types.Import{
			User:     i.username,
			Password: i.password,
			Tool:     i.tools,
		},
		Reference:    i.reference,
		ReferenceStr: i.reference.String(),
	}

	resp, err := i.client.Post(i.ctx, base.API("/v1/import"), req)
	if err != nil {
		return err
	}

	res := &types.Response{}
	if err := res.Decode(resp.Body); err != nil {
		return err
	}

	fmt.Printf("Importing %s\n", req.Reference.String())
	switch resp.StatusCode {
	case http.StatusOK:
		if !i.quiet {
			err = i.client.WatchProgress(i.ctx, res.TaskID, i.operation)
			if err != nil {
				return err
			}
			return nil
		}
	default:
		return fmt.Errorf("Failed to import artifacts: %s", res.Message)
	}

	return nil
}
