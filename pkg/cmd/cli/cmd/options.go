package cmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/templates"
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
