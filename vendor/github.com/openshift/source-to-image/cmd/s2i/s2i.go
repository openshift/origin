package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/openshift/source-to-image/pkg/cmd/cli"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	command := cli.CommandFor()
	if err := command.Execute(); err != nil {
		fmt.Println(fmt.Sprintf("S2I encountered the following error: %v", err))
		os.Exit(1)
	}
}
