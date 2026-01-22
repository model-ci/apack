package build

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
	"github.com/model-ci/apack/pkg/layerdb"
	"github.com/urfave/cli/v2"
	"oras.land/oras-go/v2/registry"
)

var Command = &cli.Command{
	Name:      "build",
	Usage:     "build files or directories into an image",
	ArgsUsage: `[OPTIONS] PATH`,
	Description: `build files or directories into an image using a build context.

The PATH specifies where to find the files for the "build context". 
PATH is a directory path (absolute or relative to the current directory).

Examples:
  apack build .                                    # build current directory
  apack build -t mymodel:latest .                  # build with tag
  apack build -t mymodel:v1.0 ./mymodel/apackfile  # build with custom apackfile
  apack build --build-arg VERSION=1.0 .            # build with build arguments`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "tag",
			Aliases: []string{"t"},
			Usage:   "Name and optionally a tag in the 'name:tag' format",
		},
		&cli.StringFlag{
			Name:    "file",
			Aliases: []string{"f"},
			Value:   "apackfile",
			Usage:   "Name of the apackfile (Default is 'PATH/apackfile')",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Suppress the build output and print image ID on success",
		},
		&cli.BoolFlag{
			Name:  "compress",
			Usage: "Compress the build context using gzip",
		},
	},
	Action: func(ctx *cli.Context) error {
		if err := utils.CheckArgs(ctx, 1, utils.ExactArgs); err != nil {
			return fmt.Errorf("%s, please specify apackfile path", err)
		}
		p, err := Newbuild(ctx)
		if err != nil {
			return err
		}
		return p.Run()
	},
}

type build struct {
	ctx       context.Context
	client    *utils.Client
	Reference registry.Reference

	BuildContext string
	apackfile    string
	ReferenceStr string

	Compress bool

	Quiet bool

	Sources     []string
	Destination string
	Exclude     []string
	Format      string
	rootConfig  string
	operation   string
	host        string
}

func Newbuild(ctx *cli.Context) (*build, error) {
	build := &build{ctx: context.Background(), operation: "build"}

	outputs := ctx.StringSlice("output")
	for _, output := range outputs {
		if !strings.Contains(output, "=") {
			return nil, fmt.Errorf("invalid output format: %s, expected format: type=local,dest=path", output)
		}
	}

	buildContext := "."
	apackfile := filepath.Join(buildContext, spec.DefaultArtifactName)
	if ctx.NArg() > 0 {
		apackfile = ctx.Args().First()
	}

	if !utils.FileExist(apackfile) {
		return nil, fmt.Errorf("build context path does not exist: %s", buildContext)
	}

	build.BuildContext = buildContext
	build.apackfile = apackfile
	build.ReferenceStr = ctx.String("tag")

	build.Compress = ctx.Bool("compress")

	build.Quiet = ctx.Bool("quiet")

	build.Sources = []string{buildContext}
	build.Destination = ""
	build.Exclude = []string{}
	build.Format = "apack"
	build.host = ctx.String("host")

	return build, build.completeAndValidate()
}

func (p *build) completeAndValidate() error {
	var err error

	if p.client == nil {
		p.client, err = utils.NewDefaultClient(p.host)
		if err != nil {
			return err
		}
	}

	if p.apackfile != filepath.Join(p.BuildContext, "Apackfile") {
		if _, err := os.Stat(p.apackfile); os.IsNotExist(err) {
			return fmt.Errorf("apackfile does not exist: %s", p.apackfile)
		}
	} else {
		if _, err := os.Stat(p.apackfile); os.IsNotExist(err) {
			return fmt.Errorf("apackfile not found at %s", p.apackfile)
		}
	}

	ref, err := registry.ParseReference(p.ReferenceStr)
	if err != nil {
		return err
	}
	p.Reference = ref

	apackignorePath := filepath.Join(p.BuildContext, spec.IgnoreFileName)
	if _, err := os.Stat(apackignorePath); err == nil {
		content, err := os.ReadFile(apackignorePath)
		if err != nil {
			return fmt.Errorf("failed to read .apackignore: %w", err)
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				p.Exclude = append(p.Exclude, line)
			}
		}
	}

	return nil
}

func (p *build) Run() error {
	args := types.Args{
		Algo: layerdb.None,
	}

	if p.Compress {
		args.Algo = layerdb.Gzip
	}

	req := types.Request{
		Reference: p.Reference,
		Args:      args,
	}

	if err := req.Artifact.UnmarshalYamlFile(p.apackfile); err != nil {
		return err
	}

	resp, err := p.client.Post(p.ctx, base.API("/v1/build"), req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	res := &types.Response{}
	if err := res.Decode(resp.Body); err != nil {
		return err
	}

	fmt.Printf("Building %s\n", req.Reference.String())
	switch resp.StatusCode {
	case http.StatusOK:
		if !p.Quiet {
			err = p.client.WatchProgress(p.ctx, res.TaskID, p.operation)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("Failed to build model: %s", res.Message)
	}

	return nil
}
