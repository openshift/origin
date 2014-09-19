package main

import (
	"fmt"
	"os"

	"github.com/openshift/origin/pkg/cmd/base"
	"github.com/openshift/origin/pkg/cmd/deployment"
	"github.com/openshift/origin/pkg/cmd/global"
	"github.com/openshift/origin/pkg/cmd/setup"
	"github.com/openshift/origin/pkg/cmd/version"
)

func main() {
	// th osc command tree
	rootCmd := base.InstallCommands(nil, []base.Cmd{
		base.Cmd{Name: "osc", ShortDescription: "Command line interface for OpenShift", LongDescription: "Command line interface for OpenShift, the hybrid Platform as a Service by the open source leader Red Hat", Children: []base.Cmd{
			base.Cmd{Name: "deployment", Executor: deployment.Main(), Children: []base.Cmd{
				base.Cmd{Name: "list", Executor: deployment.List()},
				base.Cmd{Name: "show", Executor: deployment.Show()},
				base.Cmd{Name: "create", Executor: deployment.Create()},
				base.Cmd{Name: "update", Executor: deployment.Update()},
				base.Cmd{Name: "remove", Executor: deployment.Remove()},
			}},
			base.Cmd{Name: "version", Executor: version.Main()},
			base.Cmd{Name: "setup", Executor: setup.Main()},
		},
		},
	})

	rootCmd.PersistentFlags().BoolVarP(&global.Verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&global.Raw, "raw", false, "Do not format the output from the requested operations")

	err := rootCmd.Execute()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}
