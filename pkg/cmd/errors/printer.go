package errors

import (
	"fmt"
	"io"
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
	fmt.Fprintf(out, "%v\n", err)
	if d, ok := err.(hasDetails); ok && len(d.Details()) > 0 {
		fmt.Fprintf(out, "%s\n", d.Details())
	}
	if c, ok := err.(hasCause); ok && c.Cause() != nil {
		fmt.Fprintf(out, "%v\n", c.Cause())
	}
	if s, ok := err.(hasSolution); ok && len(s.Solution()) > 0 {
		fmt.Fprintf(out, "%s", s.Solution())
	}
}
