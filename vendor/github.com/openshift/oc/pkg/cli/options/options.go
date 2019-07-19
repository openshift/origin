package options

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"
)

// NewCmdOptions implements the OpenShift cli options command
func NewCmdOptions(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use: "options",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Usage()
		},
	}

	templates.UseOptionsTemplates(cmd)

	return cmd
}
