package formatting

import (
	"fmt"

	"github.com/fatih/color"
)

func Printfln(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf(format+"\n", a...)
}

func Strong(s string) string {
	return colorize(color.Bold, s)
}

func Warn(s string) string {
	return colorize(color.FgYellow, s)
}

func Error(s string) string {
	return colorize(color.FgRed, s)
}

func Success(s string) string {
	return colorize(color.FgGreen, s)
}

func Info(s string) string {
	return colorize(color.FgBlue, s)
}

func colorize(c color.Attribute, s string) string {
	colorize := color.New(c).SprintFunc()
	return colorize(s)
}
