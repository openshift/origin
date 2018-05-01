package docker

import (
	"fmt"
	"io"

	"github.com/openshift/origin/pkg/oc/util/prefixwriter"
)

type hasCause interface {
	Cause() error
}

type hasDetails interface {
	Details() string
}

type hasSolution interface {
	Solution() string
}

func PrintError(err error, out io.Writer) {
	fmt.Fprintf(out, "Error: %v\n", err)
	if d, ok := err.(hasDetails); ok && len(d.Details()) > 0 {
		fmt.Fprintln(out, "Details:")
		w := prefixwriter.New("  ", out)
		fmt.Fprintf(w, "%s\n", d.Details())
	}
	if s, ok := err.(hasSolution); ok && len(s.Solution()) > 0 {
		fmt.Fprintln(out, "Solution:")
		w := prefixwriter.New("  ", out)
		fmt.Fprintf(w, "%s\n", s.Solution())
	}
	if c, ok := err.(hasCause); ok && c.Cause() != nil {
		fmt.Fprintln(out, "Caused By:")
		w := prefixwriter.New("  ", out)
		PrintError(c.Cause(), w)
	}
}
