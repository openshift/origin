package main

import (
	"flag"
	"os"
	"runtime"

	"github.com/golang/glog"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/logs"

	"github.com/openshift/openshift-project-reservation/pkg/cmd/server"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	stopCh := genericapiserver.SetupSignalHandler()
	cmd := server.NewCommandStartProjectReservationServer(os.Stdout, os.Stderr, stopCh)
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	if err := cmd.Execute(); err != nil {
		glog.Fatal(err)
	}
}
