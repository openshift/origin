package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/openshift/origin/pkg/cmd/dockerregistry"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

func main() {
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
