package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/origin/pkg/cmd/openshift"
)

func main() {
	basename := filepath.Base(os.Args[0])
	command := openshift.CommandFor(basename)
	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}
