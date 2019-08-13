package main

import (
	"fmt"
	"os"

	"github.com/openshift/oc/pkg/cli"
	"github.com/openshift/oc/tools/clicheck/sanity"
)

func main() {
	oc := cli.NewOcCommand("oc", "oc", os.Stdin, os.Stdout, os.Stderr)
	errors := sanity.CheckCmdTree(oc, sanity.AllCmdChecks, nil)
	if len(errors) > 0 {
		for i, err := range errors {
			fmt.Fprintf(os.Stderr, "%d. %s\n\n", i+1, err)
		}
		os.Exit(1)
	}

	fmt.Fprintln(os.Stdout, "Congrats, CLI looks good!")
}
