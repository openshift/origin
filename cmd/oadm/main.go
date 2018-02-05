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

	admCmd := ""

	// 1. there is no `oc adm version` command,special-case it here.
	// 2. oadm with no args should run oc adm and print out the default usage msg.
	if (len(os.Args) > 1 && os.Args[1] != "version") || len(os.Args) == 1 {
		admCmd = "adm"
	}

	if err := syscall.Exec(path, append([]string{baseCommand, admCmd}, os.Args[1:]...), os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
