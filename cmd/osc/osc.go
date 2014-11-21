package main

import (
	"fmt"
	"os"

	"github.com/openshift/origin/pkg/cmd/osc"
	"github.com/openshift/origin/pkg/version"
	"github.com/spf13/cobra"
)

func main() {
	// Root command
	oscCmd := osc.NewCommandDeveloper("osc")

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
