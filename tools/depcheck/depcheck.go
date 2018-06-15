package main

import (
	"fmt"
	"os"

	"github.com/openshift/origin/tools/depcheck/pkg/cmd"
)

func main() {
	command := cmd.NewCmdDepCheck(os.Args[0], os.Stdout, os.Stderr)
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
