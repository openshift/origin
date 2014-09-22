package main

import (
	"fmt"
	"os"

	"github.com/openshift/origin/pkg/cmd/deployment"
	"github.com/openshift/origin/pkg/cmd/global"
	"github.com/openshift/origin/pkg/cmd/setup"
	"github.com/openshift/origin/pkg/cmd/version"
	"github.com/spf13/cobra"
)

func main() {
	// Main osc command
	oscCmd := &cobra.Command{
		Use:   "osc",
		Short: "Command line interface for OpenShift",
		Long:  "Command line interface for OpenShift, the hybrid Platform as a Service by the open source leader Red Hat",
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	// Deployment
	deploymentCmd := deployment.NewCommandDeployment("deployment")
	deploymentCmd.AddCommand(deployment.NewCommandDeploymentList("list"))
	deploymentCmd.AddCommand(deployment.NewCommandDeploymentShow("show"))
	deploymentCmd.AddCommand(deployment.NewCommandDeploymentUpdate("update"))
	deploymentCmd.AddCommand(deployment.NewCommandDeploymentRemove("remove"))
	oscCmd.AddCommand(deploymentCmd)

	// Setup
	oscCmd.AddCommand(version.NewCommandVersion("version"))

	// Version
	oscCmd.AddCommand(setup.NewCommandSetup("setup"))

	// Global flags
	oscCmd.PersistentFlags().BoolVarP(&global.Verbose, "verbose", "v", false, "Verbose output")
	oscCmd.PersistentFlags().BoolVar(&global.Raw, "raw", false, "Do not format the output from the requested operations")

	// Root command execution path
	err := oscCmd.Execute()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}
