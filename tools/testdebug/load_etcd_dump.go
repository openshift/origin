package main

import (
	"os"
	"runtime"

	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/openshift/origin/tools/testdebug/cmd"
)

func main() {
	stopCh := genericapiserver.SetupSignalHandler()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	command := cmd.NewDebugAPIServerCommand(stopCh)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
