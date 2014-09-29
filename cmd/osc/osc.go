package main

import (
	"fmt"
	"os"

	"github.com/openshift/origin/pkg/cmd/deployment"
	"github.com/openshift/origin/pkg/cmd/setup"
	"github.com/openshift/origin/pkg/version"
	"github.com/spf13/cobra"
)

func main() {
	// Root command
	oscCmd := NewCommandOpenShiftClient("osc")

	// Version
	oscCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Command '%s' (main)", "version"),
		Long:  fmt.Sprintf("Command '%s' (main)", "version"),
		Run: func(c *cobra.Command, args []string) {
			fmt.Println("OpenShift", version.Get().String())
		},
	})

	// Root command execution path
	err := oscCmd.Execute()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}

func NewCommandOpenShiftClient(name string) *cobra.Command {
	var verbose, raw bool

	// Main command
	cmd := &cobra.Command{
		Use:   name,
		Short: "Client tools for OpenShift",
		Long:  "Client tools for OpenShift, the hybrid Platform as a Service by the open source leader Red Hat",
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	// Deployment
	cmd.AddCommand(deployment.NewCommandDeployment("deployment"))

	// Setup
	cmd.AddCommand(setup.NewCommandSetup("setup"))

	// Global flags
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	cmd.PersistentFlags().BoolVar(&raw, "raw", false, "Do not format the output from the requested operations")

	return cmd
}
