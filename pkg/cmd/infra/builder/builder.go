package builder

import (
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/build/builder/cmd"
	"github.com/openshift/origin/pkg/version"
)

const (
	stiBuilderLong = `Perform a Source-to-Image Build.

This command executes a Source-to-Image build using arguments passed via the environment.
It expects to be run inside of a container.`

	dockerBuilderLong = `Perform a Docker Build.

This command executes a Docker build using arguments passed via the environment.
It expects to be run inside of a container.`
)

// NewCommandSTIBuilder provides a CLI handler for STI build type
func NewCommandSTIBuilder(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Run an OpenShift Source-to-Images build",
		Long:  stiBuilderLong,
		Run: func(c *cobra.Command, args []string) {
			cmd.RunSTIBuild()
		},
	}

	cmd.AddCommand(version.NewVersionCommand(name))
	return cmd
}

// NewCommandDockerBuilder provides a CLI handler for Docker build type
func NewCommandDockerBuilder(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Run an OpenShift Docker build",
		Long:  dockerBuilderLong,
		Run: func(c *cobra.Command, args []string) {
			cmd.RunDockerBuild()
		},
	}
	cmd.AddCommand(version.NewVersionCommand(name))
	return cmd
}
