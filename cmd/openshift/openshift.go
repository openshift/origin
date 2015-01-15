package main

import (
	"os"
	"path/filepath"

	"github.com/openshift/origin/pkg/cmd/openshift"
)

func main() {
	basename := filepath.Base(os.Args[0])
	command := openshift.CommandFor(basename)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
