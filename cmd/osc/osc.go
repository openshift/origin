package main

import (
	"fmt"
	"os"

	"github.com/openshift/origin/pkg/cmd/auth"
	"github.com/openshift/origin/pkg/cmd/create"
	"github.com/openshift/origin/pkg/cmd/deployment"
	"github.com/openshift/origin/pkg/cmd/pod"
	"github.com/openshift/origin/pkg/cmd/setup"
	"github.com/openshift/origin/pkg/version"
	"github.com/spf13/cobra"
)

const longDescription = `
End-user client tool for OpenShift, the hybrid Platform as a Service by the open source leader Red Hat.

Note: This is an alpha release of OpenShift and will change significantly.  See

    https://github.com/openshift/origin

for the latest information on OpenShift.

`

// TODO: rebase kubectl and refactor to the new commands structures
func main() {
	// Root command
	oscCmd := NewCmdOpenShiftClient("osc")

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
	if err := oscCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err)
		os.Exit(1)
	}
}

func NewCmdOpenShiftClient(name string) *cobra.Command {
	// Main command
	cmd := &cobra.Command{
		Use:   name,
		Short: "Client tools for OpenShift",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	// Subcommands
	cmd.AddCommand(create.NewCmdCreate("create"))
	cmd.AddCommand(deployment.NewCmdDeployment("deployment"))
	cmd.AddCommand(pod.NewCmdPod("pod"))
	cmd.AddCommand(setup.NewCmdSetup("setup"))
	cmd.AddCommand(auth.NewCmdLogin("login"))
	cmd.AddCommand(auth.NewCmdLogout("logout"))

	return cmd
}
