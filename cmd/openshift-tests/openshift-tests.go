package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/cmd/monitor_command"
	"github.com/openshift/origin/pkg/cmd/monitor_command/timeline"
	"github.com/openshift/origin/pkg/cmd/openshift-tests/images"
	"github.com/openshift/origin/pkg/cmd/openshift-tests/run"
	run_test "github.com/openshift/origin/pkg/cmd/openshift-tests/run-test"
	run_upgrade "github.com/openshift/origin/pkg/cmd/openshift-tests/run-upgrade"
	"github.com/openshift/origin/pkg/monitor/resourcewatch/cmd"
	"github.com/openshift/origin/pkg/riskanalysis"
	testginkgo "github.com/openshift/origin/pkg/test/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
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
		run.NewRunCommand(ioStreams),
		run_upgrade.NewRunUpgradeCommand(ioStreams),
		images.NewImagesCommand(),
		run_test.NewRunTestCommand(ioStreams),
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
