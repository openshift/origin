package main

import (
	"flag"
	"log"
	"os"

	"github.com/mesos/mesos-go/api/v1/cmd/example-scheduler/app"
)

func main() {
	cfg := app.NewConfig()
	fs := flag.NewFlagSet("scheduler", flag.ExitOnError)
	cfg.AddFlags(fs)
	fs.Parse(os.Args[1:])

	if err := app.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
