package main

import (
	"fmt"
	"os"

	"github.com/openshift/origin/pkg/cmd/commands"
	"github.com/openshift/origin/pkg/cmd/util/formatting"
	"github.com/spf13/cobra"
)

func main() {
	var Verbose, Raw bool

	cmd := &cobra.Command{
		Use:   "osc",
		Short: "Command line interface for OpenShift",
		Long:  formatting.Strong("Command line interface for OpenShift, the hybrid Platform as a Service by the open source leader Red Hat"),
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	cmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "Verbose output")
	cmd.PersistentFlags().BoolVar(&Raw, "raw", false, "Do not format the output from the requested operations")

	commands.InstallCommand(cmd, "setup", commands.Setup)
	commands.InstallCommand(cmd, "deployments", commands.Deployments)
	commands.InstallCommand(cmd, "projects", commands.Projects)

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}
