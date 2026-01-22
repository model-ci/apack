package gen

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/model-ci/apack/internal/api/base"
	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/internal/utils"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:      "gen",
	Usage:     "Generate a apackfile for the contents of a directory",
	ArgsUsage: `[directory]`,
	Description: `Examine the contents of a directory and attempt to generate a basic apackfile
based on common file formats. Any files whose type (i.e. model, dataset, etc.)
cannot be determined will be included in a code layer.

By default the command will prompt for input for a name and description for the apackfile.

Examples:
# Generate a apackfile for the current directory:
apack gen .

# Generate a apackfile for files in ./my-model, with name "mymodel" and a description:
apack gen --name "mymodel" --desc "This is my model's description" ./my-model

# Generate a apackfile, overwriting any existing apackfile:
apack gen ./mymodel --force

# Complete example with all parameters:
apack gen --name "my-ai-model" --desc "Transformer-based text classification model" --author "John Doe" ./model-dir --force
`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "name",
			Usage: "Model name",
		},
		&cli.StringFlag{
			Name:  "desc",
			Usage: "Description for the model",
		},
		&cli.StringSliceFlag{
			Name:  "author",
			Usage: "Author for the model",
		},
		&cli.BoolFlag{
			Name:  "force",
			Usage: "Overwrite existing apackfile if present",
		},
	},
	Action: func(ctx *cli.Context) error {
		if err := utils.CheckArgs(ctx, 1, utils.ExactArgs); err != nil {
			return fmt.Errorf("%s, please specify model file path", err)
		}
		init, err := NewGen(ctx)
		if err != nil {
			return err
		}
		return init.Run()
	},
}

type Gen struct {
	ctx       context.Context
	client    *utils.Client
	workspace string
	overwrite bool
	name      string
	desc      string
	author    []string
	host      string
}

func NewGen(ctx *cli.Context) (*Gen, error) {
	gen := &Gen{ctx: context.Background()}

	gen.workspace = ctx.Args().First()
	gen.overwrite = ctx.Bool("force")
	gen.author = ctx.StringSlice("author")
	gen.host = ctx.String("host")
	return gen, gen.completeAndValidate()
}

func (g *Gen) completeAndValidate() error {
	var err error
	g.client, err = utils.NewDefaultClient(g.host)
	if err != nil {
		return err
	}
	if g.name == "" {
		name, err := utils.PromptForInput("Enter a name for the Model: ", false)
		if err != nil {
			return err
		}
		g.name = name
	}
	if g.desc == "" {
		desc, err := utils.PromptForInput("Enter a short description for the Model: ", false)
		if err != nil {
			return err
		}
		g.desc = desc
	}
	if g.author == nil {
		author, err := utils.PromptForInput("Enter an author for the Model: ", false)
		if err != nil {
			return err
		}
		g.author = strings.Split(author, ",")
	}

	if !filepath.IsAbs(g.workspace) {
		g.workspace, err = filepath.Abs(g.workspace)
	}

	return nil
}

func (g *Gen) Run() error {
	artifact := spec.Artifact{}
	if g.name != "" || g.desc != "" {
		artifact.Spec.Descriptor.Name = g.name
		artifact.Spec.Descriptor.Description = g.desc
	}

	if g.author != nil {
		artifact.Spec.Descriptor.Authors = append(artifact.Spec.Descriptor.Authors, g.author...)
	}

	if g.workspace != "" {
		artifact.Package.Workspace = g.workspace
	}

	req := types.Request{Artifact: artifact, Args: types.Args{Overwrite: g.overwrite}}

	resp, err := g.client.Post(g.ctx, base.API("/v1/gen"), req)
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
		artifactPath := filepath.Join(g.workspace, spec.DefaultArtifactName)
		if err := os.WriteFile(artifactPath, res.Message, 0644); err != nil {
			return fmt.Errorf("Failed to write artifact: %s", err)
		}
		fmt.Println("Generated apackfile:")
		fmt.Println("-------")
		fmt.Println(string(res.Message))

	default:
		return fmt.Errorf("Failed to generate apackfile: %s", res.Message)
	}

	return nil
}
