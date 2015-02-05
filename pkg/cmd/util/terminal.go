package util

import (
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/pkg/term"
	"github.com/golang/glog"
)

func PromptForString(r io.Reader, format string, a ...interface{}) string {
	fmt.Printf(format, a...)
	return readInput(r)
}

// TODO not tested on other platforms
func PromptForPasswordString(r io.Reader, format string, a ...interface{}) string {
	if file, ok := r.(*os.File); ok {
		inFd := file.Fd()

		if term.IsTerminal(inFd) {
			oldState, err := term.SaveState(inFd)
			if err != nil {
				glog.V(3).Infof("Unable to save terminal state")
				return PromptForString(r, format, a...)
			}

			fmt.Printf(format, a...)

			term.DisableEcho(inFd, oldState)

			input := readInput(r)

			defer term.RestoreTerminal(inFd, oldState)

			fmt.Printf("\n")

			return input
		} else {
			glog.V(3).Infof("Stdin is not a terminal")
			return PromptForString(r, format, a...)
		}
	} else {
		glog.V(3).Infof("Unable to use a TTY")
		return PromptForString(r, format, a...)
	}
}

func readInput(r io.Reader) string {
	var result string
	fmt.Fscan(r, &result)
	return result
}
