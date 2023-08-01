package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/cmd/monitor_command"
	"github.com/openshift/origin/pkg/cmd/monitor_command/timeline"
	"github.com/openshift/origin/pkg/monitor/resourcewatch/cmd"
	"github.com/openshift/origin/pkg/riskanalysis"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/pkg/testsuites"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"
)

func main() {
	// KUBE_TEST_REPO_LIST is calculated during package initialization and prevents
	// proper mirroring of images referenced by tests. Clear the value and re-exec the
	// current process to ensure we can verify from a known state.
	if len(os.Getenv("KUBE_TEST_REPO_LIST")) > 0 {
		fmt.Fprintln(os.Stderr, "warning: KUBE_TEST_REPO_LIST may not be set when using openshift-tests and will be ignored")
		os.Setenv("KUBE_TEST_REPO_LIST", "")
		// resolve the call to execute since Exec() does not do PATH resolution
		if err := syscall.Exec(exec.Command(os.Args[0]).Path, os.Args, os.Environ()); err != nil {
			panic(fmt.Sprintf("%s: %v", os.Args[0], err))
		}
		return
	}

	logs.InitLogs()
	defer logs.FlushLogs()

	logrus.SetLevel(logrus.InfoLevel)

	rand.Seed(time.Now().UTC().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	//pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	root := &cobra.Command{
		Long: templates.LongDesc(`
		OpenShift Tests

		This command verifies behavior of an OpenShift cluster by running remote tests against
		the cluster API that exercise functionality. In general these tests may be disruptive
		or require elevated privileges - see the descriptions of each test suite.
		`),
	}

	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	root.AddCommand(
		newRunCommand(ioStreams),
		newRunUpgradeCommand(ioStreams),
		newImagesCommand(),
		newRunTestCommand(ioStreams),
		newDevCommand(),
		monitor_command.NewRunMonitorCommand(ioStreams),
		monitor_command.NewMonitorCommand(),
		newTestFailureRiskAnalysisCommand(),
		cmd.NewRunResourceWatchCommand(),
		timeline.NewTimelineCommand(ioStreams),
		NewRunInClusterDisruptionMonitorCommand(ioStreams),
	)

	f := flag.CommandLine.Lookup("v")
	root.PersistentFlags().AddGoFlag(f)
	pflag.CommandLine = pflag.NewFlagSet("empty", pflag.ExitOnError)
	flag.CommandLine = flag.NewFlagSet("empty", flag.ExitOnError)
	exutil.InitStandardFlags()

	if err := func() error {
		defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()
		return root.Execute()
	}(); err != nil {
		if ex, ok := err.(testginkgo.ExitError); ok {
			fmt.Fprintf(os.Stderr, "Ginkgo exit error %d: %v\n", ex.Code, err)
			os.Exit(ex.Code)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

const sippyDefaultURL = "https://sippy.dptools.openshift.org/api/jobs/runs/risk_analysis"

func newTestFailureRiskAnalysisCommand() *cobra.Command {
	riskAnalysisOpts := &riskanalysis.Options{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	cmd := &cobra.Command{
		Use:   "risk-analysis",
		Short: "Performs risk analysis on test failures",
		Long: templates.LongDesc(`
Uses the test failure summary json files written along-side our junit xml
files after an invocation of openshift-tests. If multiple files are present
(multiple invocations of openshift-tests) we will merge them into one.
Results are then submitted to sippy which will return an analysis of per-test
and overall risk level given historical pass rates on the failed tests.
The resulting analysis is then also written to the junit artifacts directory.
`),

		RunE: func(cmd *cobra.Command, args []string) error {
			return riskAnalysisOpts.Run()
		},
	}
	cmd.Flags().StringVar(&riskAnalysisOpts.JUnitDir,
		"junit-dir", riskAnalysisOpts.JUnitDir,
		"The directory where test reports were written, and analysis file will be stored.")
	cmd.MarkFlagRequired("junit-dir")
	cmd.Flags().StringVar(&riskAnalysisOpts.SippyURL,
		"sippy-url", sippyDefaultURL,
		"Sippy URL API endpoint")
	return cmd
}

type imagesOptions struct {
	Repository string
	Upstream   bool
	Verify     bool
}

func newImagesCommand() *cobra.Command {
	o := &imagesOptions{}
	cmd := &cobra.Command{
		Use:   "images",
		Short: "Gather images required for testing",
		Long: templates.LongDesc(fmt.Sprintf(`
		Creates a mapping to mirror test images to a private registry

		This command identifies the locations of all test images referenced by the test
		suite and outputs a mirror list for use with 'oc image mirror' to copy those images
		to a private registry. The list may be passed via file or standard input.

				$ openshift-tests images --to-repository private.com/test/repository > /tmp/mirror
				$ oc image mirror -f /tmp/mirror

		The 'run' and 'run-upgrade' subcommands accept '--from-repository' which will source
		required test images from your mirror.

		See the help for 'oc image mirror' for more about mirroring to disk or consult the docs
		for mirroring offline. You may use a file:// prefix in your '--to-repository', but when
		mirroring from disk to your offline repository you will have to construct the appropriate
		disk to internal registry statements yourself.

		By default, the test images are sourced from a public container image repository at
		%[1]s and are provided as-is for testing purposes only. Images are mirrored by the project
		to the public repository periodically.
		`, defaultTestImageMirrorLocation)),

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.Verify {
				return verifyImages()
			}

			repository := o.Repository
			var prefix string
			for _, validPrefix := range []string{"file://", "s3://"} {
				if strings.HasPrefix(repository, validPrefix) {
					repository = strings.TrimPrefix(repository, validPrefix)
					prefix = validPrefix
					break
				}
			}
			ref, err := reference.Parse(repository)
			if err != nil {
				return fmt.Errorf("--to-repository is not valid: %v", err)
			}
			if len(ref.Tag) > 0 || len(ref.ID) > 0 {
				return fmt.Errorf("--to-repository may not include a tag or image digest")
			}

			if err := verifyImages(); err != nil {
				return err
			}
			lines, err := createImageMirrorForInternalImages(prefix, ref, !o.Upstream)
			if err != nil {
				return err
			}
			for _, line := range lines {
				fmt.Fprintln(os.Stdout, line)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&o.Upstream, "upstream", o.Upstream, "Retrieve images from the default upstream location")
	cmd.Flags().StringVar(&o.Repository, "to-repository", o.Repository, "A container image repository to mirror to.")
	// this is a private flag for debugging only
	cmd.Flags().BoolVar(&o.Verify, "verify", o.Verify, "Verify the contents of the image mappings")
	cmd.Flags().MarkHidden("verify")
	return cmd
}

func newRunCommand(streams genericclioptions.IOStreams) *cobra.Command {
	f := NewRunSuiteFlags(streams, defaultTestImageMirrorLocation, testsuites.StandardTestSuites())

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
	f.BindOptions(cmd.Flags())
	return cmd
}

func newRunUpgradeCommand(streams genericclioptions.IOStreams) *cobra.Command {
	f := NewRunUpgradeSuiteFlags(streams, defaultTestImageMirrorLocation, testsuites.UpgradeTestSuites())

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

	f.BindOptions(cmd.Flags())
	return cmd
}

func newRunTestCommand(streams genericclioptions.IOStreams) *cobra.Command {
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

			if err := verifyImagesWithoutEnv(); err != nil {
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
			upgradeOptions, err := NewUpgradeOptionsFromYAML(upgradeOptionsYAML)
			if err != nil {
				return err
			}
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
