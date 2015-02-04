package cmd

import (
	"io"

	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

func NewCmdOptions(f *Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options",
		Short: "List options that can be passed to any command",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.SetUsageTemplate(templates.OptionsUsageTemplate)
	cmd.SetHelpTemplate("")

	return cmd
}
