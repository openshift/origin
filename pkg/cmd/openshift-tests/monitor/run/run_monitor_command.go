package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/test/extended/util/image"

	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/defaultmonitortests"
	"github.com/openshift/origin/pkg/disruption/backend/sampler"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
)

type RunMonitorFlags struct {
	ArtifactDir    string
	DisplayFromNow bool

	genericclioptions.IOStreams
}

func NewRunMonitorOptions(streams genericclioptions.IOStreams) *RunMonitorFlags {
	return &RunMonitorFlags{
		DisplayFromNow: true,
		IOStreams:      streams,
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
	f := NewRunMonitorOptions(streams)

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
	flags.StringVar(&f.ArtifactDir, "artifact-dir", f.ArtifactDir, "The directory where monitor events will be stored.")
	flags.BoolVar(&f.DisplayFromNow, "display-from-now", f.DisplayFromNow, "Only display intervals from at or after this comand was started.")
}

func (f *RunMonitorFlags) ToOptions() (*RunMonitorOptions, error) {
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

	return &RunMonitorOptions{
		ArtifactDir:     f.ArtifactDir,
		DisplayFilterFn: displayFilterFn,
		IOStreams:       f.IOStreams,
	}, nil
}

type RunMonitorOptions struct {
	ArtifactDir     string
	DisplayFilterFn monitorapi.EventIntervalMatchesFunc

	genericclioptions.IOStreams
}

// Run starts monitoring the cluster by invoking Start, periodically printing the
// events accumulated to Out. When the user hits CTRL+C or signals termination the
// condition intervals (all non-instantaneous events) are reported to Out.
func (f *RunMonitorOptions) Run() error {
	fmt.Fprintf(f.Out, "Starting the monitor.\n")

	image.InitializeImages(imagesetup.DefaultTestImageMirrorLocation)
	restConfig, err := monitor.GetMonitorRESTConfig()
	if err != nil {
		return err
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal, 2)
	go func() {
		<-abortCh
		fmt.Fprintf(f.ErrOut, "Interrupted, terminating\n")
		sampler.TearDownInClusterMonitors(restConfig)
		cancelFn()

		sig := <-abortCh
		fmt.Fprintf(f.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	monitorTestInfo := monitortestframework.MonitorTestInitializationInfo{
		ClusterStabilityDuringTest: monitortestframework.Stable,
	}
	recorder := monitor.WrapWithJSONLRecorder(monitor.NewRecorder(), f.Out, f.DisplayFilterFn)
	m := monitor.NewMonitor(
		recorder,
		restConfig,
		f.ArtifactDir,
		defaultmonitortests.NewMonitorTestsFor(monitorTestInfo),
	)
	if err := m.Start(ctx); err != nil {
		return err
	}
	fmt.Fprintf(f.Out, "Monitor started, waiting for ctrl+C to stop...\n")

	<-ctx.Done()

	fmt.Fprintf(f.Out, "Monitor shutting down, this may take up to five minutes...\n")

	cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cleanupCancel()
	// ignore the ResultState because we're interested in whether we collected, not whether what we collected passed.
	if _, err := m.Stop(cleanupContext); err != nil {
		fmt.Fprintf(os.Stderr, "error cleaning up, still reporting as best as possible: %v\n", err)
	}

	// Store events to artifact directory
	if err := m.SerializeResults(ctx, "invariants", ""); err != nil {
		return err
	}

	return nil
}
