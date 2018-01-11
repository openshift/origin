package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

var baseCommand = "oc"

func main() {
	if runtime.GOOS == "windows" {
		baseCommand = strings.ToLower(baseCommand) + ".exe"
	}

	path, err := exec.LookPath(baseCommand)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "DEPRECATED: The 'oadm' command is deprecated, please use '%s adm' instead.\n", baseCommand)

	if err := syscall.Exec(path, append([]string{"adm"}, os.Args[1:]...), os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
