package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/openshift/origin/pkg/clioptions/clusterinfo"

	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/monitortestframework"
	exutil "github.com/openshift/origin/test/extended/util"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/test/extended/util/image"

	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/defaultmonitortests"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
)

type RunMonitorFlags struct {
	ArtifactDir         string
	DisplayFromNow      bool
	ExactMonitorTests   []string
	DisableMonitorTests []string
	FromRepository      string

	genericclioptions.IOStreams
}

func NewRunMonitorOptions(streams genericclioptions.IOStreams, fromRepository string) *RunMonitorFlags {
	return &RunMonitorFlags{
		DisplayFromNow: true,
		IOStreams:      streams,
		FromRepository: fromRepository,
	}
}

// TODO remove
func NewRunMonitorCommand(streams genericclioptions.IOStreams) *cobra.Command {
	return newRunCommand("run-monitor", streams)
}

func NewRunCommand(streams genericclioptions.IOStreams) *cobra.Command {
	return newRunCommand("run", streams)
}

func newRunCommand(name string, streams genericclioptions.IOStreams) *cobra.Command {
	f := NewRunMonitorOptions(streams, imagesetup.DefaultTestImageMirrorLocation)

	cmd := &cobra.Command{
		Use:   name,
		Short: "Continuously verify the cluster is functional",
		Long: templates.LongDesc(`
		Run a continuous verification process

		`),

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			o, err := f.ToOptions()
			if err != nil {
				return err
			}
			return o.Run()
		},
	}

	f.BindFlags(cmd.Flags())

	return cmd
}

func (f *RunMonitorFlags) BindFlags(flags *pflag.FlagSet) {
	monitorNames := defaultmonitortests.ListAllMonitorTests()

	flags.StringVar(&f.ArtifactDir, "artifact-dir", f.ArtifactDir, "The directory where monitor events will be stored.")
	flags.BoolVar(&f.DisplayFromNow, "display-from-now", f.DisplayFromNow, "Only display intervals from at or after this comand was started.")
	flags.StringSliceVar(&f.ExactMonitorTests, "monitor", f.ExactMonitorTests,
		fmt.Sprintf("list of exactly which monitors to enable. All others will be disabled.  Current monitors are: [%s]", strings.Join(monitorNames, ", ")))
	flags.StringSliceVar(&f.DisableMonitorTests, "disable-monitor", f.DisableMonitorTests, "list of monitors to disable.  Defaults for others will be honored.")
	flags.StringVar(&f.FromRepository, "from-repository", f.FromRepository, "A container image repository to retrieve test images from.")
}

func (f *RunMonitorFlags) ToOptions() (*RunMonitorOptions, error) {
	// This is to set testsStarted = true to avoid panic
	exutil.WithCleanup(func() {})

	var displayFilterFn monitorapi.EventIntervalMatchesFunc
	if f.DisplayFromNow {
		now := time.Now()
		displayFilterFn = func(eventInterval monitorapi.Interval) bool {
			if eventInterval.From.IsZero() {
				return true
			}
			return eventInterval.From.After(now)
		}
	}

	monitorTestRegistry, err := f.getMonitorTestRegistry()
	if err != nil {
		return nil, err
	}

	return &RunMonitorOptions{
		ArtifactDir:     f.ArtifactDir,
		DisplayFilterFn: displayFilterFn,
		MonitorTests:    monitorTestRegistry,
		IOStreams:       f.IOStreams,
		FromRepository:  f.FromRepository,
	}, nil
}

func (f *RunMonitorFlags) getMonitorTestRegistry() (monitortestframework.MonitorTestRegistry, error) {
	monitorTestInfo := monitortestframework.MonitorTestInitializationInfo{
		ClusterStabilityDuringTest: monitortestframework.Stable,
		ExactMonitorTests:          f.ExactMonitorTests,
		DisableMonitorTests:        f.DisableMonitorTests,
	}
	return defaultmonitortests.NewMonitorTestsFor(monitorTestInfo)
}

type RunMonitorOptions struct {
	ArtifactDir     string
	DisplayFilterFn monitorapi.EventIntervalMatchesFunc
	MonitorTests    monitortestframework.MonitorTestRegistry
	FromRepository  string

	genericclioptions.IOStreams
}

// Run starts monitoring the cluster by invoking Start, periodically printing the
// events accumulated to Out. When the user hits CTRL+C or signals termination the
// condition intervals (all non-instantaneous events) are reported to Out.
func (o *RunMonitorOptions) Run() error {
	// set globals so that helpers will create pods with the mapped images if we create them from this process.
	image.InitializeImages(o.FromRepository)

	fmt.Fprintf(o.Out, "Starting the monitor.\n")

	restConfig, err := clusterinfo.GetMonitorRESTConfig()
	if err != nil {
		return err
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal, 2)
	go func() {
		<-abortCh
		fmt.Fprintf(o.ErrOut, "Interrupted, terminating\n")
		cancelFn()

		sig := <-abortCh
		fmt.Fprintf(o.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	recorder := monitor.WrapWithJSONLRecorder(monitor.NewRecorder(), o.Out, o.DisplayFilterFn)
	m := monitor.NewMonitor(
		recorder,
		restConfig,
		o.ArtifactDir,
		o.MonitorTests,
	)
	if err := m.Start(ctx); err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "Monitor started, waiting for ctrl+C to stop...\n")

	<-ctx.Done()

	fmt.Fprintf(o.Out, "Monitor shutting down, this may take up to twenty minutes...\n")

	cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cleanupCancel()
	// ignore the ResultState because we're interested in whether we collected, not whether what we collected passed.
	if _, err := m.Stop(cleanupContext); err != nil {
		fmt.Fprintf(os.Stderr, "error cleaning up, still reporting as best as possible: %v\n", err)
	}

	// Store events to artifact directory
	if _, err := m.SerializeResults(ctx, "invariants", ""); err != nil {
		return err
	}

	return nil
}
