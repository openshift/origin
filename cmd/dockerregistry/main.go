package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/openshift/origin/pkg/cmd/dockerregistry"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/serviceability"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

func main() {
	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"))()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()
	startProfiler()

	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()

	// TODO convert to flags instead of a config file?
	configurationPath := ""
	if flag.NArg() > 0 {
		configurationPath = flag.Arg(0)
	}
	if configurationPath == "" {
		configurationPath = os.Getenv("REGISTRY_CONFIGURATION_PATH")
	}

	if configurationPath == "" {
		fmt.Println("configuration path unspecified")
		os.Exit(1)
	}
	// Prevent a warning about unrecognized environment variable
	os.Unsetenv("REGISTRY_CONFIGURATION_PATH")

	configFile, err := os.Open(configurationPath)
	if err != nil {
		log.Fatalf("Unable to open configuration file: %s", err)
	}

	dockerregistry.Execute(configFile)
}

func startProfiler() {
	if cmdutil.Env("OPENSHIFT_PROFILE", "") == "web" {
		go func() {
			runtime.SetBlockProfileRate(1)
			profile_port := cmdutil.Env("OPENSHIFT_PROFILE_PORT", "6060")
			log.Infof(fmt.Sprintf("Starting profiling endpoint at http://127.0.0.1:%s/debug/pprof/", profile_port))
			log.Fatal(http.ListenAndServe(fmt.Sprintf("127.0.0.1:%s", profile_port), nil))
		}()
	}
}
