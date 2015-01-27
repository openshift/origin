package main

import (
	"os"
	"path/filepath"

	"github.com/openshift/origin/pkg/cmd/openshift"
	"github.com/openshift/origin/pkg/cmd/util/serviceability"
)

func main() {
	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"))()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	basename := filepath.Base(os.Args[0])
	command := openshift.CommandFor(basename)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
