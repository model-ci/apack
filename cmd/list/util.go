package list

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/internal/types"
	"github.com/model-ci/apack/pkg/progress"
	"oras.land/oras-go/v2/registry"
)

const (
	listTableHeader = "REPOSITORY\tTAG\tMAINTAINER\tNAME\tSIZE\tDIGEST"
	listTableFmt    = "%s\t%s\t%s\t%s\t%s\t%s"
)

func printSummary(w io.Writer, artifacts types.Artifacts) {
	var lines []string
	for _, img := range artifacts.Items {
		lines = append(lines, format(img)...)
	}
	tw := tabwriter.NewWriter(w, 0, 2, 3, ' ', 0)
	fmt.Fprintln(tw, listTableHeader)
	for _, line := range lines {
		fmt.Fprintln(tw, line)
	}
	tw.Flush()
}

func format(artifact spec.Artifact) []string {
	pkg := artifact.Package
	desc := artifact.Spec.Descriptor
	ref, err := registry.ParseReference(pkg.Reference)
	if err != nil {
		panic(err)
	}
	if ref.Reference == "" {
		line := fmt.Sprintf(listTableFmt, ref.Registry, "<none>", desc.Authors, desc.Name, pkg.Size, pkg.Digest)
		return []string{line}
	}

	var lines []string
	line := fmt.Sprintf(listTableFmt, ref.Registry+"/"+ref.Repository, ref.Reference, desc.Authors, desc.Name, progress.FormatSize(pkg.Size), pkg.Digest)
	lines = append(lines, line)
	return lines
}
