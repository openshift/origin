package run

import (
	"context"
	"fmt"

	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/testsuites"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewRunCommand(streams genericclioptions.IOStreams) *cobra.Command {
	f := NewRunSuiteFlags(streams, imagesetup.DefaultTestImageMirrorLocation, testsuites.StandardTestSuites())

	cmd := &cobra.Command{
		Use:   "run SUITE",
		Short: "Run a test suite",
		Long: templates.LongDesc(`
		Run a test suite against an OpenShift server

		This command will run one of the following suites against a cluster identified by the current
		KUBECONFIG file. See the suite description for more on what actions the suite will take.

		If you specify the --dry-run argument, the names of each individual test that is part of the
		suite will be printed, one per line. You may filter this list and pass it back to the run
		command with the --file argument. You may also pipe a list of test names, one per line, on
		standard input by passing "-f -".

		`) + testsuites.SuitesString(testsuites.StandardTestSuites(), "\n\nAvailable test suites:\n\n"),

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := f.ToOptions(args)
			if err != nil {
				fmt.Fprintf(f.IOStreams.ErrOut, "error converting to options: %v", err)
				return err
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := o.Run(ctx); err != nil {
				fmt.Fprintf(f.IOStreams.ErrOut, "error running options: %v", err)
				return err
			}
			return nil
		},
	}
	f.BindFlags(cmd.Flags())
	return cmd
}
