package builder

import (
	"os"

	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/build/builder/cmd"
	ocmd "github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/templates"
)

var (
	s2iBuilderLong = templates.LongDesc(`
		Perform a Source-to-Image build

		This command executes a Source-to-Image build using arguments passed via the environment.
		It expects to be run inside of a container.`)

	dockerBuilderLong = templates.LongDesc(`
		Perform a Docker build

		This command executes a Docker build using arguments passed via the environment.
		It expects to be run inside of a container.`)
)

// NewCommandS2IBuilder provides a CLI handler for S2I build type
func NewCommandS2IBuilder(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Run a Source-to-Image build",
		Long:  s2iBuilderLong,
		Run: func(c *cobra.Command, args []string) {
			err := cmd.RunS2IBuild(c.OutOrStderr())
			kcmdutil.CheckErr(err)
		},
	}

	cmd.AddCommand(ocmd.NewCmdVersion(name, nil, os.Stdout, ocmd.VersionOptions{}))
	return cmd
}

// NewCommandDockerBuilder provides a CLI handler for Docker build type
func NewCommandDockerBuilder(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Run a Docker build",
		Long:  dockerBuilderLong,
		Run: func(c *cobra.Command, args []string) {
			err := cmd.RunDockerBuild(c.OutOrStderr())
			kcmdutil.CheckErr(err)
		},
	}
	cmd.AddCommand(ocmd.NewCmdVersion(name, nil, os.Stdout, ocmd.VersionOptions{}))
	return cmd
}
