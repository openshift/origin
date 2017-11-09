package main

import (
	"flag"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/logs"

	webconsolecmd "github.com/openshift/origin/pkg/assets/apiserver/cmd"
	"github.com/openshift/origin/pkg/cmd/util/serviceability"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	logs.InitLogs()
	defer logs.FlushLogs()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"))()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	cmd := webconsolecmd.NewCommandStartWebConsoleServer(os.Stdout, os.Stderr, wait.NeverStop)
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	if err := cmd.Execute(); err != nil {
		glog.Fatal(err)
	}
}
