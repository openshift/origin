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
	out.Init(buf, 0, 8, 1, '\t', 0)
	f(out)
	out.Flush()
	str := string(buf.String())
	return str
}

type scale struct {
	scale uint64
	unit  string
}

var (
	mega = scale{20, "MiB"}
	giga = scale{30, "GiB"}
)

func printSize(out io.Writer, size int64) {
	scale := mega
	if size >= (1 << 30) {
		scale = giga
	}
	integer := size >> scale.scale
	// fraction is the reminder of a division shifted by one order of magnitude
	fraction := (size % (1 << scale.scale)) >> (scale.scale - 10)
	// additionally we present only 2 digits after dot, so divide by 10
	fraction = fraction / 10
	if fraction > 0 {
		printValue(out, fmt.Sprintf("%d.%02d%s", integer, fraction, scale.unit))
	} else {
		printValue(out, fmt.Sprintf("%d%s", integer, scale.unit))
	}
}
