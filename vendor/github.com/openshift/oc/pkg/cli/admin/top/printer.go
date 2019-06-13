package top

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

type Info interface {
	PrintLine(out io.Writer)
}

func Print(out io.Writer, headers []string, infos []Info) {
	s := tabbedString(func(out *tabwriter.Writer) {
		printHeader(out, headers)
		for _, info := range infos {
			info.PrintLine(out)
			fmt.Fprintf(out, "\n")
		}
	})
	fmt.Fprintf(out, "%s", s)
}

func printHeader(out io.Writer, columns []string) {
	for _, col := range columns {
		printValue(out, col)
	}
	fmt.Fprintf(out, "\n")
}

func printArray(out io.Writer, values []string) {
	if len(values) == 0 {
		printValue(out, "<none>")
	} else {
		printValue(out, strings.Join(values, ", "))
	}
}

func printValue(out io.Writer, value interface{}) {
	fmt.Fprintf(out, "%v\t", value)
}

func printBool(out io.Writer, value bool) {
	if value {
		printValue(out, "yes")
	} else {
		printValue(out, "no")
	}
}

func tabbedString(f func(*tabwriter.Writer)) string {
	out := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	out.Init(buf, 0, 8, 1, ' ', 0)
	f(out)
	out.Flush()
	str := string(buf.String())
	return str
}
