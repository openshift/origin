package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/openshift/origin/pkg/cmd/dockerregistry"
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
		fmt.Println("configuration path unspecified")
		os.Exit(1)
	}

	configFile, err := os.Open(configurationPath)
	if err != nil {
		log.Fatalf("Unable to open configuration file: %s", err)
	}

	dockerregistry.Execute(configFile)
}
