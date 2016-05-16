package builder

import (
	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/build/builder/cmd"
	"github.com/openshift/origin/pkg/version"
)

const (
	stiBuilderLong = `
Perform a Source-to-Image build

This command executes a Source-to-Image build using arguments passed via the environment.
It expects to be run inside of a container.`

	dockerBuilderLong = `
Perform a Docker build

This command executes a Docker build using arguments passed via the environment.
It expects to be run inside of a container.`
)

// NewCommandSTIBuilder provides a CLI handler for STI build type
func NewCommandSTIBuilder(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Run a Source-to-Image build",
		Long:  stiBuilderLong,
		Run: func(c *cobra.Command, args []string) {
			err := cmd.RunSTIBuild(c.Out())
			kcmdutil.CheckErr(err)
		},
	}

	cmd.AddCommand(version.NewVersionCommand(name, false))
	return cmd
}

// NewCommandDockerBuilder provides a CLI handler for Docker build type
func NewCommandDockerBuilder(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Run a Docker build",
		Long:  dockerBuilderLong,
		Run: func(c *cobra.Command, args []string) {
			err := cmd.RunDockerBuild(c.Out())
			kcmdutil.CheckErr(err)
		},
	}
	cmd.AddCommand(version.NewVersionCommand(name, false))
	return cmd
}
