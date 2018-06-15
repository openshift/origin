package main

import (
	"os"

	"github.com/openshift/origin/tools/buildanalyzer/cmd"
)

func main() {

	command := cmd.NewBuildAnalyzerCommand()
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
