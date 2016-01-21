package main

import (
	"os"
	"runtime"

	"github.com/openshift/origin/tools/testdebug/cmd"
)

func main() {
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	command := cmd.NewDebugAPIServerCommand()
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
