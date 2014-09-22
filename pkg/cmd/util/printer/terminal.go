package printer

import (
	"fmt"
)

type TerminalPrinter struct{}

func (v *TerminalPrinter) Print(i ...interface{}) {
	fmt.Print(i...)
}

func (v *TerminalPrinter) Println(i ...interface{}) {
	fmt.Println(i...)
}

func (v *TerminalPrinter) Warn(i ...interface{}) {
	fmt.Print(i...)
}

func (v *TerminalPrinter) Warnln(i ...interface{}) {
	fmt.Println(i...)
}

func (v *TerminalPrinter) Error(i ...interface{}) {
	fmt.Print(i...)
}

func (v *TerminalPrinter) Errorln(i ...interface{}) {
	fmt.Println(i...)
}

func (v *TerminalPrinter) Success(i ...interface{}) {
	fmt.Print(i...)
}

func (v *TerminalPrinter) Successln(i ...interface{}) {
	fmt.Println(i...)
}
