package run

import (
	"context"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/cmd"
	"github.com/openshift/origin/pkg/testsuites"
)

func NewRunCommand(streams genericclioptions.IOStreams, internalExtension *extension.Extension) *cobra.Command {
	f := NewRunSuiteFlags(streams, imagesetup.DefaultTestImageMirrorLocation)

	runCmd := &cobra.Command{
		Use:   "run SUITE",
		Short: "Run a test suite",
		Long: templates.LongDesc(`
		Run a test suite against an OpenShift server

		This command will run one of the available test suites against a cluster identified by the current
		KUBECONFIG file. See the suite description for more on what actions the suite will take.

		If you specify the --dry-run argument, the names of each individual test that is part of the
		suite will be printed, one per line. You may filter this list and pass it back to the run
		command with the --file argument. You may also pipe a list of test names, one per line, on
		standard input by passing "-f -".

		Use 'openshift-tests list suites' to see all available test suites.
		`),

		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE:       cmd.RequireClusterAccess,
		RunE: func(cmd *cobra.Command, args []string) error {
			allSuites, err := testsuites.AllTestSuites(context.Background())
			if err != nil {
				return errors.WithMessage(err, "couldn't retrieve test suites")
			}

			o, err := f.ToOptions(args, allSuites, internalExtension)
			if err != nil {
				return errors.WithMessage(err, "error converting to options")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if err := o.Run(ctx); err != nil {
				return errors.WithMessage(err, "error running a test suite")
			}
			return nil
		},
	}
	f.BindFlags(runCmd.Flags())
	return runCmd
}
