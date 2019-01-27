package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"

	"github.com/onsi/gomega"

	"github.com/golang/glog"
	"github.com/onsi/ginkgo"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apiserver/pkg/util/logs"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/monitor"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	rand.Seed(time.Now().UTC().UnixNano())

	root := &cobra.Command{
		Long: templates.LongDesc(`
		OpenShift Tests

		This command verifies behavior of an OpenShift cluster by running remote tests against
		the cluster API that exercise functionality. In general these tests may be disruptive
		or require elevated privileges - see the descriptions of each test suite.
		`),
	}
	flagtypes.GLog(root.PersistentFlags())

	suites := staticSuites

	suiteOpt := &testginkgo.Options{
		Suites: suites,
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

		`) + testginkgo.SuitesString(suites, "\n\nAvailable test suites:\n\n"),

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var exitErr error
			var out, errOut io.Writer = os.Stdout, os.Stderr
			if len(suiteOpt.OutFile) > 0 {
				f, err := os.OpenFile(suiteOpt.OutFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0640)
				if err != nil {
					return err
				}
				defer func() {
					if exitErr != nil {
						fmt.Fprintf(f, "error: %s", exitErr)
					}
					if err := f.Close(); err != nil {
						fmt.Fprintf(os.Stderr, "error: Unable to close output file\n")
					}
				}()
				out = io.MultiWriter(out, f)
				errOut = io.MultiWriter(errOut, f)
			}
			suiteOpt.Out, suiteOpt.ErrOut = out, errOut

			if exitErr = initProvider(suiteOpt.Provider); exitErr != nil {
				return exitErr
			}
			os.Setenv("TEST_PROVIDER", suiteOpt.Provider)
			exitErr = suiteOpt.Run(args)
			return exitErr
		},
	}
	cmd.Flags().BoolVar(&suiteOpt.DryRun, "dry-run", suiteOpt.DryRun, "Print the tests to run without executing them.")
	cmd.Flags().StringVar(&suiteOpt.JUnitDir, "junit-dir", suiteOpt.JUnitDir, "The directory to write test reports to.")
	cmd.Flags().StringVar(&suiteOpt.Provider, "provider", suiteOpt.Provider, "The cluster infrastructure provider. Will automatically default to the correct value.")
	cmd.Flags().StringVarP(&suiteOpt.TestFile, "file", "f", suiteOpt.TestFile, "Create a suite from the newline-delimited test names in this file.")
	cmd.Flags().StringVarP(&suiteOpt.OutFile, "output-file", "o", suiteOpt.OutFile, "Write all test output to this file.")
	cmd.Flags().DurationVar(&suiteOpt.Timeout, "timeout", suiteOpt.Timeout, "Set the maximum time a test can run before being aborted. This is read from the suite by default, but will be 10 minutes otherwise.")
	cmd.Flags().BoolVar(&suiteOpt.IncludeSuccessOutput, "include-success", suiteOpt.IncludeSuccessOutput, "Print output from successful tests.")
	root.AddCommand(cmd)

	testOpt := &testginkgo.TestOptions{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	cmd = &cobra.Command{
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
			if err := initProvider(os.Getenv("TEST_PROVIDER")); err != nil {
				return err
			}
			return testOpt.Run(args)
		},
	}
	cmd.Flags().BoolVar(&testOpt.DryRun, "dry-run", testOpt.DryRun, "Print the test to run without executing them.")
	root.AddCommand(cmd)

	monitorOpt := &monitor.Options{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	cmd = &cobra.Command{
		Use:   "run-monitor",
		Short: "Continuously verify the cluster is functional",
		Long: templates.LongDesc(`
		Run a continuous verification process

		`),

		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return monitorOpt.Run()
		},
	}
	root.AddCommand(cmd)

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

func initProvider(provider string) error {
	// record the exit error to the output file
	if err := decodeProviderTo(provider, exutil.TestContext); err != nil {
		return err
	}
	exutil.TestContext.AllowedNotReadyNodes = 100
	exutil.TestContext.MaxNodesToGather = 0

	exutil.AnnotateTestSuite()
	exutil.InitTest()
	gomega.RegisterFailHandler(ginkgo.Fail)

	// TODO: infer SSH keys from the cluster
	return nil
}

func decodeProviderTo(provider string, testContext *e2e.TestContextType) error {
	switch provider {
	case "":
		if _, ok := os.LookupEnv("KUBE_SSH_USER"); ok {
			if _, ok := os.LookupEnv("LOCAL_SSH_KEY"); ok {
				testContext.Provider = "local"
			}
		}
		// TODO: detect which provider the cluster is running and use that as a default.
	default:
		var providerInfo struct{ Type string }
		if err := json.Unmarshal([]byte(provider), &providerInfo); err != nil {
			return fmt.Errorf("provider must be a JSON object with the 'type' key at a minimum: %v", err)
		}
		if len(providerInfo.Type) == 0 {
			return fmt.Errorf("provider must be a JSON object with the 'type' key")
		}
		testContext.Provider = providerInfo.Type
		if err := json.Unmarshal([]byte(provider), &testContext.CloudConfig); err != nil {
			return fmt.Errorf("provider must decode into the cloud config object: %v", err)
		}
	}
	glog.V(2).Infof("Provider %s: %#v", testContext.Provider, testContext.CloudConfig)
	return nil
}
