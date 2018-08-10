package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile
var gitCommit = ""

// version will be populated by the Makefile, read from
// VERSION file of the source code.
var version = ""

func main() {
	app := cli.NewApp()
	app.Name = "oci-runtime-tool"
	if gitCommit != "" {
		app.Version = fmt.Sprintf("%s, commit: %s", version, gitCommit)
	} else {
		app.Version = version
	}
	app.Usage = "OCI (Open Container Initiative) runtime tools"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "compliance-level",
			Value: "must",
			Usage: "compliance level (may, should, or must).",
		},
		cli.BoolFlag{
			Name:  "host-specific",
			Usage: "generate host-specific configs or do host-specific validations",
		},
		cli.StringFlag{
			Name:  "log-level",
			Value: "error",
			Usage: "Log level (panic, fatal, error, warn, info, or debug)",
		},
	}

	app.Commands = []cli.Command{
		generateCommand,
		bundleValidateCommand,
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func before(context *cli.Context) error {
	logLevelString := context.GlobalString("log-level")
	logLevel, err := logrus.ParseLevel(logLevelString)
	if err != nil {
		logrus.Fatalf(err.Error())
	}
	logrus.SetLevel(logLevel)

	return nil
}
