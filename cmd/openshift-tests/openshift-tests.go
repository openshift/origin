package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/openshift-eng/openshift-tests-extension/pkg/cmd/cmdinfo"
	"github.com/openshift-eng/openshift-tests-extension/pkg/cmd/cmdrun"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/origin/pkg/cmd"
	collectdiskcertificates "github.com/openshift/origin/pkg/cmd/openshift-tests/collect-disk-certificates"
	"github.com/openshift/origin/pkg/cmd/openshift-tests/dev"
	"github.com/openshift/origin/pkg/cmd/openshift-tests/disruption"
	e2e_analysis "github.com/openshift/origin/pkg/cmd/openshift-tests/e2e-analysis"
	"github.com/openshift/origin/pkg/cmd/openshift-tests/images"
	"github.com/openshift/origin/pkg/cmd/openshift-tests/list"
	"github.com/openshift/origin/pkg/cmd/openshift-tests/monitor"
	run_monitor "github.com/openshift/origin/pkg/cmd/openshift-tests/monitor/run"
	"github.com/openshift/origin/pkg/cmd/openshift-tests/monitor/timeline"
	"github.com/openshift/origin/pkg/cmd/openshift-tests/render"
	risk_analysis "github.com/openshift/origin/pkg/cmd/openshift-tests/risk-analysis"
	"github.com/openshift/origin/pkg/cmd/openshift-tests/run"
	run_disruption "github.com/openshift/origin/pkg/cmd/openshift-tests/run-disruption"
	run_upgrade "github.com/openshift/origin/pkg/cmd/openshift-tests/run-upgrade"
	"github.com/openshift/origin/pkg/cmd/openshift-tests/run_resource_watch"
	versioncmd "github.com/openshift/origin/pkg/cmd/openshift-tests/version"
	"github.com/openshift/origin/pkg/test/extensions"
	exutil "github.com/openshift/origin/test/extended/util"
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

	// The GCE PD drivers were removed in kube 1.31, so we can ignore the env var that
	// some automation sets.
	if os.Getenv("ENABLE_STORAGE_GCE_PD_DRIVER") != "" {
		logrus.Warn("ENABLE_STORAGE_GCE_PD_DRIVER is set, but is not supported")
		os.Unsetenv("ENABLE_STORAGE_GCE_PD_DRIVER")
	}

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	//pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)

	extensionRegistry, originExtension, err := extensions.InitializeOpenShiftTestsExtensionFramework()
	if err != nil {
		panic(err)
	}

	root := &cobra.Command{
		Long: templates.LongDesc(`This command verifies behavior of an OpenShift cluster by running remote tests against
		the cluster API that exercise functionality. In general these tests may be disruptive
		or require elevated privileges - see the descriptions of each test suite.
		`),
		// PersistentPreRun to always print the openshift-tests version; this populates
		// down to subcommands as well. If you need to omit this output for a specific command,
		// you can override PersistentPreRun to NoPrintVersion instead.
		PersistentPreRun: cmd.PrintVersion,
	}

	ioStreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	root.AddCommand(
		run.NewRunCommand(ioStreams, originExtension),
		list.NewListCommand(ioStreams, extensionRegistry),
		cmdinfo.NewInfoCommand(extensionRegistry),
		run_upgrade.NewRunUpgradeCommand(ioStreams),
		images.NewImagesCommand(),
		cmdrun.NewRunTestCommand(extensionRegistry),
		dev.NewDevCommand(),
		run_monitor.NewRunMonitorCommand(ioStreams),
		monitor.NewMonitorCommand(ioStreams),
		disruption.NewDisruptionCommand(ioStreams),
		risk_analysis.NewTestFailureRiskAnalysisCommand(),
		e2e_analysis.NewTestFailureClusterAnalysisCheckCommand(),
		run_resource_watch.NewRunResourceWatchCommand(),
		timeline.NewTimelineCommand(ioStreams),
		run_disruption.NewRunInClusterDisruptionMonitorCommand(ioStreams),
		collectdiskcertificates.NewRunCollectDiskCertificatesCommand(ioStreams),
		render.NewRenderCommand(ioStreams),
		versioncmd.NewVersionCommand(ioStreams),
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
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
