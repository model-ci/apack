package diff

import (
	"context"
	"fmt"

	"github.com/model-ci/apack/cmd/inspect"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/pkg/client"
	"github.com/urfave/cli/v2"
	"oras.land/oras-go/v2/registry"
)

var Command = &cli.Command{
	Name:      "diff",
	Usage:     "Compare two artifacts to see their differences",
	ArgsUsage: `ARTIFACTS1[:TAG] ARTIFACTS2[:TAG]`,
	Description: `Compare two artifacts to see their differences in configuration, layers, and annotations.

Examples:
  apack diff mymodel:v1.0 mymodel:v2.0           # Compare different versions
  apack diff model1:latest model2:latest         # Compare different contents`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "host",
			Aliases: []string{"H"},
			Usage:   "Registry server address",
		},
	},
	Action: func(ctx *cli.Context) error {
		diff, err := NewDiff(ctx)
		if err != nil {
			return err
		}
		return diff.Run()
	},
}

type Diff struct {
	ctx       context.Context
	operation string
	diffA     registry.Reference
	diffB     registry.Reference
	host      string
	client    *client.BaseClient
}

func NewDiff(ctx *cli.Context) (*Diff, error) {
	diff := &Diff{ctx: context.Background(), operation: "diff"}
	diff.host = ctx.String("host")

	args := ctx.Args().Slice()
	if len(args) != 2 {
		return nil, fmt.Errorf("requires exactly 2 arguments: ARTIFACTS1[:TAG] ARTIFACTS2[:TAG]")
	}

	refA, err := registry.ParseReference(args[0])
	if err != nil {
		return nil, fmt.Errorf("invalid first artifact reference %q: %w", args[0], err)
	}
	diff.diffA = refA

	refB, err := registry.ParseReference(args[1])
	if err != nil {
		return nil, fmt.Errorf("invalid second artifact reference %q: %w", args[1], err)
	}
	diff.diffB = refB

	return diff, diff.completeAndValidate()
}

func (d *Diff) completeAndValidate() error {
	var err error
	if d.client == nil {
		d.client, err = client.NewDefaultCLI(d.host)
		if err != nil {
			return err
		}
	}

	if d.diffA.String() == d.diffB.String() {
		return fmt.Errorf("cannot compare identical artifacts")
	}

	return nil
}

func (d *Diff) Run() error {
	ins1, ins2, err := inspectPair(d.ctx, d.client, d.diffA, d.diffB)
	if err != nil {
		return err
	}
	if ins1.Artifact.Package.Digest == ins2.Artifact.Package.Digest {
		fmt.Println("Artifacts are identical")
		return nil
	}

	result := CompareManifests(&ins1.Manifest.Manifest, &ins2.Manifest.Manifest)

	fmt.Println("Comparing:")
	fmt.Printf("  Artifact1: %s\n", d.diffA.String())
	fmt.Printf("  Artifact2: %s\n\n", d.diffB.String())

	fmt.Println("Configs:")
	fmt.Println("---------------------------------------")
	if result.SameConfig {
		fmt.Printf("  Configs are identical (Digest: %s)\n\n", ins1.Manifest.Config.Digest[:17])

	} else {
		fmt.Printf("Configs differ:\n")
		fmt.Printf("  Artifact1 Config Digest: %s\n", ins1.Manifest.Config.Digest[:17])
		fmt.Printf("  Artifact2 Config Digest: %s\n\n", ins2.Manifest.Config.Digest[:17])
	}

	fmt.Println("Annotations:")
	fmt.Println("---------------------------------------")
	if result.AnnotationsMatch {
		fmt.Printf("  Annotations are identical \n\n")
	} else {
		fmt.Printf("  Annotations does not match\n\n")
	}

	displayLayers("Shared Layers", result.SharedLayers)
	displayLayers(fmt.Sprintf("Unique Layers to Artifact1 (%s)", d.diffA.String()), result.UniqueLayersA)
	displayLayers(fmt.Sprintf("Unique Layers to Artifact2 (%s)", d.diffB.String()), result.UniqueLayersB)
	return nil

}

func inspectPair(ctx context.Context, cli *client.BaseClient, ref1, ref2 registry.Reference) (*types.Inspect, *types.Inspect, error) {
	inspect1, err := inspect.Insepection(ctx, cli, ref1)
	if err != nil {
		return nil, nil, err
	}

	inspect2, err := inspect.Insepection(ctx, cli, ref2)
	if err != nil {
		return nil, nil, err
	}

	return inspect1, inspect2, nil
}
