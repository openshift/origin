package run_test

import (
	"os"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/clioptions/upgradeoptions"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewRunTestCommand(streams genericclioptions.IOStreams) *cobra.Command {
	testOpt := testginkgo.NewTestOptions(streams)

	cmd := &cobra.Command{
		Use:   "run-test NAME",
		Short: "Run a single test by name",
		Long: templates.LongDesc(`
		Execute a single test

		This executes a single test by name. It is used by the run command during suite execution but may also
		be used to test in isolation while developing new tests.
		`),

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if v := os.Getenv("TEST_LOG_LEVEL"); len(v) > 0 {
				cmd.Flags().Lookup("v").Value.Set(v)
			}

			// set globals so that helpers will create pods with the mapped images if we create them from this process.
			// we cannot eliminate the env var usage until we convert run-test, which we may be able to do in a followup.
			image.InitializeImages(os.Getenv("KUBE_TEST_REPO"))

			if err := imagesetup.VerifyImages(); err != nil {
				return err
			}

			config, err := clusterdiscovery.DecodeProvider(os.Getenv("TEST_PROVIDER"), testOpt.DryRun, false, nil)
			if err != nil {
				return err
			}
			if err := clusterdiscovery.InitializeTestFramework(exutil.TestContext, config, testOpt.DryRun); err != nil {
				return err
			}
			klog.V(4).Infof("Loaded test configuration: %#v", exutil.TestContext)

			exutil.TestContext.ReportDir = os.Getenv("TEST_JUNIT_DIR")

			// allow upgrade test to pass some parameters here, although this may be
			// better handled as an env var within the test itself in the future
			upgradeOptionsYAML := os.Getenv("TEST_UPGRADE_OPTIONS")
			upgradeOptions, err := upgradeoptions.NewUpgradeOptionsFromYAML(upgradeOptionsYAML)
			if err != nil {
				return err
			}
			// TODO this is called from run-upgrade and run-test.  At least one of these ought not need it.
			if err := upgradeOptions.SetUpgradeGlobals(); err != nil {
				return err
			}

			exutil.WithCleanup(func() { err = testOpt.Run(args) })
			return err
		},
	}
	cmd.Flags().BoolVar(&testOpt.DryRun, "dry-run", testOpt.DryRun, "Print the test to run without executing them.")
	return cmd
}
