package printer

import (
	"fmt"
	"os"
)

type TerminalPrinter struct{}

func (v *TerminalPrinter) Printf(s string, i ...interface{}) {
	fmt.Printf(s+"\n", i...)
}

func (v *TerminalPrinter) Warnf(s string, i ...interface{}) {
	fmt.Printf(s+"\n", i...)
}

func (v *TerminalPrinter) Errorf(s string, i ...interface{}) {
	fmt.Fprintf(os.Stderr, s+"\n", i...)
}

func (v *TerminalPrinter) Successf(s string, i ...interface{}) {
	fmt.Printf(s+"\n", i...)
}
