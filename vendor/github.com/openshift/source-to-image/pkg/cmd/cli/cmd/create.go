package cmd

import (
	"github.com/spf13/cobra"

	"github.com/openshift/source-to-image/pkg/create"
)

// NewCmdCreate implements the S2I cli create command.
func NewCmdCreate() *cobra.Command {
	return &cobra.Command{
		Use:   "create <imageName> <destination>",
		Short: "Bootstrap a new S2I image repository",
		Long:  "Bootstrap a new S2I image with given imageName inside the destination directory",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 2 {
				cmd.Help()
				return
			}
			b := create.New(args[0], args[1])
			b.AddSTIScripts()
			b.AddDockerfile()
			b.AddReadme()
			b.AddTests()
		},
	}
}
