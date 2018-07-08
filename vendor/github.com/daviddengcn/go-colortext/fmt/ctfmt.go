// Package ctfmt is a handful wrapping of go-colortext (ct) package and fmt package.
package ctfmt

import (
	"fmt"

	"github.com/daviddengcn/go-colortext"
)

// Print calls fmt.Print with foreground color set.
func Print(cl ct.Color, bright bool, a ...interface{}) (n int, err error) {
	ct.Foreground(cl, bright)
	defer ct.ResetColor()

	return fmt.Print(a...)
}

// Println calls fmt.Println with foreground color set.
func Println(cl ct.Color, bright bool, a ...interface{}) (n int, err error) {
	ct.Foreground(cl, bright)
	defer ct.ResetColor()

	return fmt.Println(a...)
}

// Printf calls fmt.Printf with foreground color set.
func Printf(cl ct.Color, bright bool, format string, a ...interface{}) (n int, err error) {
	ct.Foreground(cl, bright)
	defer ct.ResetColor()

	return fmt.Printf(format, a...)
}

// Printfln calls fmt.Printf and add an extra new-line char with foreground color set.
func Printfln(cl ct.Color, bright bool, format string, a ...interface{}) (n int, err error) {
	ct.Foreground(cl, bright)
	defer ct.ResetColor()

	return fmt.Printf(format+"\n", a...)
}
