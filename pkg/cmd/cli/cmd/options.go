package cmd

import (
	"io"

	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

func NewCmdOptions(f *Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use: "options",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.SetUsageTemplate(templates.OptionsUsageTemplate())
	cmd.SetHelpTemplate(templates.OptionsHelpTemplate())

	return cmd
}
