package cmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

func NewCmdOptions(f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use: "options",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	templates.UseOptionsTemplates(cmd)

	return cmd
}
