package list

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewListCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available resources",
		Long: templates.LongDesc(`
		List various resources available in openshift-tests.

		This command provides subcommands to list different types of resources
		such as test suites, tests, and other information.
		`),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		NewListSuitesCommand(streams),
		NewListExtensionsCommand(streams),
	)

	return cmd
}
