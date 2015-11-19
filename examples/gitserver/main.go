package main

import (
	"fmt"
	"log"
	"os"

	"github.com/openshift/origin/pkg/gitserver"
)

func main() {
	if len(os.Args) != 1 {
		fmt.Printf(`git-server - Expose Git repositories to the network

%[1]s`, gitserver.EnvironmentHelp)
		os.Exit(0)
	}
	config, err := gitserver.NewEnviromentConfig()
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(gitserver.Start(config))
}
