package main

import (
	"flag"
	goflag "flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/resourcewatch/cmd"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/pkg/version"
	exutil "github.com/openshift/origin/test/extended/util"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	rand.Seed(time.Now().UTC().UnixNano())

	// UGLY HACK TO TEST CI
	sa := os.Getenv("GCP_SHARED_CREDENTIALS_FILE")
	os.Setenv("E2E_GOOGLE_APPLICATION_CREDENTIALS", sa)

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	root := &cobra.Command{
		Long: templates.LongDesc(`
		OpenShift Tests

		This command verifies behavior of an OpenShift cluster by running remote tests against
		the cluster API that exercise functionality. In general these tests may be disruptive
		or require elevated privileges - see the descriptions of each test suite.
		`),
	}

	root.AddCommand(
		newRunCommand(),
		newRunUpgradeCommand(),
		newRunTestCommand(),
		newRunMonitorCommand(),
		cmd.NewRunResourceWatchCommand(),
	)

	pflag.CommandLine = pflag.NewFlagSet("empty", pflag.ExitOnError)
	flag.CommandLine = flag.NewFlagSet("empty", flag.ExitOnError)
	exutil.InitStandardFlags()

	if err := func() error {
		defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()
		return root.Execute()
	}(); err != nil {
		if ex, ok := err.(testginkgo.ExitError); ok {
			os.Exit(ex.Code)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRunMonitorCommand() *cobra.Command {
	monitorOpt := &monitor.Options{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	cmd := &cobra.Command{
		Use:   "run-monitor",
		Short: "Continuously verify the cluster is functional",
		Long: templates.LongDesc(`
		Run a continuous verification process

		`),

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return monitorOpt.Run()
		},
	}
	return cmd
}

func newRunCommand() *cobra.Command {
	opt := &testginkgo.Options{
		Suites: staticSuites,
	}

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

		`) + testginkgo.SuitesString(opt.Suites, "\n\nAvailable test suites:\n\n"),

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mirrorToFile(opt, func() error {
				if !opt.DryRun {
					fmt.Fprintf(os.Stderr, "%s version: %s\n", filepath.Base(os.Args[0]), version.Get().String())
				}
				config, err := decodeProvider(opt.Provider, opt.DryRun, true)
				if err != nil {
					return err
				}
				opt.Provider = config.ToJSONString()
				matchFn, err := initializeTestFramework(exutil.TestContext, config, opt.DryRun)
				if err != nil {
					return err
				}
				opt.MatchFn = matchFn
				return opt.Run(args)
			})
		},
	}
	bindOptions(opt, cmd.Flags())
	return cmd
}

func newRunUpgradeCommand() *cobra.Command {
	opt := &testginkgo.Options{Suites: upgradeSuites}
	upgradeOpt := &UpgradeOptions{}

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

		`) + testginkgo.SuitesString(opt.Suites, "\n\nAvailable upgrade suites:\n\n"),

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mirrorToFile(opt, func() error {
				if len(upgradeOpt.ToImage) == 0 {
					return fmt.Errorf("--to-image must be specified to run an upgrade test")
				}

				if len(args) > 0 {
					for _, suite := range opt.Suites {
						if suite.Name == args[0] {
							upgradeOpt.Suite = suite.Name
							upgradeOpt.JUnitDir = opt.JUnitDir
							value := upgradeOpt.ToEnv()
							if _, err := initUpgrade(value); err != nil {
								return err
							}
							opt.SuiteOptions = value
							break
						}
					}
				}

				config, err := decodeProvider(opt.Provider, opt.DryRun, true)
				if err != nil {
					return err
				}
				opt.Provider = config.ToJSONString()
				matchFn, err := initializeTestFramework(exutil.TestContext, config, opt.DryRun)
				if err != nil {
					return err
				}
				opt.MatchFn = matchFn
				return opt.Run(args)
			})
		},
	}

	bindOptions(opt, cmd.Flags())
	bindUpgradeOptions(upgradeOpt, cmd.Flags())
	return cmd
}

func newRunTestCommand() *cobra.Command {
	testOpt := &testginkgo.TestOptions{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

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
			upgradeOpts, err := initUpgrade(os.Getenv("TEST_SUITE_OPTIONS"))
			if err != nil {
				return err
			}
			config, err := decodeProvider(os.Getenv("TEST_PROVIDER"), testOpt.DryRun, false)
			if err != nil {
				return err
			}
			if _, err := initializeTestFramework(exutil.TestContext, config, testOpt.DryRun); err != nil {
				return err
			}
			exutil.TestContext.ReportDir = upgradeOpts.JUnitDir
			exutil.WithCleanup(func() { err = testOpt.Run(args) })
			return err
		},
	}
	cmd.Flags().BoolVar(&testOpt.DryRun, "dry-run", testOpt.DryRun, "Print the test to run without executing them.")
	return cmd
}

// mirrorToFile ensures a copy of all output goes to the provided OutFile, including
// any error returned from fn. The function returns fn() or any error encountered while
// attempting to open the file.
func mirrorToFile(opt *testginkgo.Options, fn func() error) error {
	if opt.Out == nil {
		opt.Out = os.Stdout
	}
	if opt.ErrOut == nil {
		opt.ErrOut = os.Stderr
	}
	if len(opt.OutFile) == 0 {
		return fn()
	}

	f, err := os.OpenFile(opt.OutFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	opt.Out = io.MultiWriter(opt.Out, f)
	opt.ErrOut = io.MultiWriter(opt.ErrOut, f)
	exitErr := fn()
	if exitErr != nil {
		fmt.Fprintf(f, "error: %s", exitErr)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintf(opt.ErrOut, "error: Unable to close output file\n")
	}
	return exitErr
}

func bindOptions(opt *testginkgo.Options, flags *pflag.FlagSet) {
	flags.BoolVar(&opt.DryRun, "dry-run", opt.DryRun, "Print the tests to run without executing them.")
	flags.BoolVar(&opt.PrintCommands, "print-commands", opt.PrintCommands, "Print the sub-commands that would be executed instead.")
	flags.StringVar(&opt.JUnitDir, "junit-dir", opt.JUnitDir, "The directory to write test reports to.")
	flags.StringVar(&opt.Provider, "provider", opt.Provider, "The cluster infrastructure provider. Will automatically default to the correct value.")
	flags.StringVarP(&opt.TestFile, "file", "f", opt.TestFile, "Create a suite from the newline-delimited test names in this file.")
	flags.StringVar(&opt.Regex, "run", opt.Regex, "Regular expression of tests to run.")
	flags.StringVarP(&opt.OutFile, "output-file", "o", opt.OutFile, "Write all test output to this file.")
	flags.IntVar(&opt.Count, "count", opt.Count, "Run each test a specified number of times. Defaults to 1 or the suite's preferred value.")
	flags.DurationVar(&opt.Timeout, "timeout", opt.Timeout, "Set the maximum time a test can run before being aborted. This is read from the suite by default, but will be 10 minutes otherwise.")
	flags.BoolVar(&opt.IncludeSuccessOutput, "include-success", opt.IncludeSuccessOutput, "Print output from successful tests.")
	flags.IntVar(&opt.Parallelism, "max-parallel-tests", opt.Parallelism, "Maximum number of tests running in parallel. 0 defaults to test suite recommended value, which is different in each suite.")
}
