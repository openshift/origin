package list

import (
	"fmt"

	"github.com/openshift/origin/pkg/testsuites"
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
	)

	return cmd
}

func NewListSuitesCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suites",
		Short: "List available test suites",
		Long: templates.LongDesc(`
		List all available test suites that can be run with the 'run' command.

		This command displays the names and descriptions of all test suites available
		in openshift-tests. Use the suite names with the 'run' command to execute
		specific test suites.
		`),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			suites := testsuites.StandardTestSuites()
			output := testsuites.SuitesString(suites, "Available test suites:\n\n")
			fmt.Fprint(streams.Out, output)
			return nil
		},
	}

	return cmd
}
