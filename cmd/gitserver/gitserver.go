package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"k8s.io/apiserver/pkg/util/logs"

	"github.com/openshift/origin/pkg/cmd/infra/gitserver"
	"github.com/openshift/origin/pkg/cmd/util/serviceability"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()
	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"))()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	rand.Seed(time.Now().UTC().UnixNano())
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	basename := filepath.Base(os.Args[0])
	command := gitserver.CommandFor(basename)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
