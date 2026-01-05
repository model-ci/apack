package ps

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/model-ci/apack/internal/runtime"
)

const (
	psTableHeader = "RUNTIME ID\tARTIFACT\tCREATE\tSTATUS\tENDPOINTS\tNAMES"
	psTableFmt    = "%s\t%s\t%s\t%s\t%s\t%s"
)

func printSummary(w io.Writer, states runtime.States) {

	var lines []string
	for _, state := range states.Items {
		lines = append(lines, format(state)...)
	}
	tw := tabwriter.NewWriter(w, 0, 2, 3, ' ', 0)
	fmt.Fprintln(tw, psTableHeader)
	for _, line := range lines {
		fmt.Fprintln(tw, line)
	}
	tw.Flush()
}

func format(state runtime.State) []string {
	var lines []string
	line := fmt.Sprintf(psTableFmt, state.ID, state.ModelImage, state.CreateAt,
		state.Status, sliceToStr(state.Endpoints), sliceToStr(state.Names))
	lines = append(lines, line)
	return lines
}

func sliceToStr(ss []string) string {
	l := len(ss)
	switch l {
	case 0:
		return "<none>"
	case 1:
		return ss[0]
	default:
		return strings.Join(ss, ", ")
	}
}
