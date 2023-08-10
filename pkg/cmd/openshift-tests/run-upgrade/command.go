package run_upgrade

import (
	"context"
	"fmt"

	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/testsuites"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewRunUpgradeCommand(streams genericclioptions.IOStreams) *cobra.Command {
	f := NewRunUpgradeSuiteFlags(streams, imagesetup.DefaultTestImageMirrorLocation, testsuites.UpgradeTestSuites())

	cmd := &cobra.Command{
		Use:   "run-upgrade SUITE",
		Short: "Run an upgrade suite",
		Long: templates.LongDesc(`
		Run an upgrade test suite against an OpenShift server

		This command will run one of the following suites against a cluster identified by the current
		KUBECONFIG file. See the suite description for more on what actions the suite will take.

		If you specify the --dry-run argument, the actions the suite will take will be printed to the
		output.

		Supported options:

		* abort-at=NUMBER - Set to a number between 0 and 100 to control the percent of operators
		at which to stop the current upgrade and roll back to the current version.
		* disrupt-reboot=POLICY - During upgrades, periodically reboot master nodes. If set to 'graceful'
		the reboot will allow the node to shut down services in an orderly fashion. If set to 'force' the
		machine will terminate immediately without clean shutdown.

		`) + testsuites.SuitesString(testsuites.UpgradeTestSuites(), "\n\nAvailable upgrade suites:\n\n"),

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
