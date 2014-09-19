package formatting

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/openshift/origin/pkg/cmd/global"
)

func Printfln(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf(format+LineBreak(), a...)
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

func LineBreak() string {
	return "\n"
}

func BR() string {
	return LineBreak()
}

func Paragraph(content string) string {
	return LineBreak() + LineBreak() + content + LineBreak() + LineBreak() //ouch
}

func P(content string) string {
	return Paragraph(content)
}

func Tab() string {
	return "\t"
}

func colorize(c color.Attribute, s string) string {
	if global.Raw {
		return s
	} else {
		colorize := color.New(c).SprintFunc()
		return colorize(s)
	}
}
