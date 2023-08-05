package monitor_command

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openshift/origin/pkg/cmd/monitor_command/timeline"

	"github.com/openshift/origin/pkg/defaultinvariants"

	"github.com/openshift/origin/pkg/disruption/backend/sampler"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption/controlplane"
	"github.com/openshift/origin/test/extended/util/disruption/externalservice"
	"github.com/openshift/origin/test/extended/util/disruption/frontends"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
)

// RunMonitorOptions is used to run a monitoring process against the provided server as
// a command line interaction.
type RunMonitorOptions struct {
	Out, ErrOut io.Writer
	ArtifactDir string

	TimelineOptions timeline.TimelineOptions
}

func NewRunMonitorOptions(ioStreams genericclioptions.IOStreams) *RunMonitorOptions {
	timelineOptions := timeline.NewTimelineOptions(ioStreams)

	return &RunMonitorOptions{
		Out:             ioStreams.Out,
		ErrOut:          ioStreams.ErrOut,
		TimelineOptions: *timelineOptions,
	}
}

func NewRunMonitorCommand(ioStreams genericclioptions.IOStreams) *cobra.Command {
	monitorOpt := NewRunMonitorOptions(ioStreams)
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
	cmd.Flags().StringVar(&monitorOpt.ArtifactDir,
		"artifact-dir", monitorOpt.ArtifactDir,
		"The directory where monitor events will be stored.")
	return cmd
}

// Run starts monitoring the cluster by invoking Start, periodically printing the
// events accumulated to Out. When the user hits CTRL+C or signals termination the
// condition intervals (all non-instantaneous events) are reported to Out.
func (opt *RunMonitorOptions) Run() error {
	restConfig, err := monitor.GetMonitorRESTConfig()
	if err != nil {
		return err
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal, 2)
	go func() {
		<-abortCh
		fmt.Fprintf(opt.ErrOut, "Interrupted, terminating\n")
		sampler.TearDownInClusterMonitors(restConfig)
		cancelFn()
		sig := <-abortCh
		fmt.Fprintf(opt.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	recorder := monitor.NewRecorder()
	m := monitor.NewMonitor(
		recorder,
		restConfig,
		opt.ArtifactDir,
		[]monitor.StartEventIntervalRecorderFunc{
			controlplane.StartAPIMonitoringUsingNewBackend,
			frontends.StartAllIngressMonitoring,
			externalservice.StartExternalServiceMonitoring,
		},
		defaultinvariants.NewInvariantsFor(defaultinvariants.Stable),
	)
	if err := m.Start(ctx); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		var last time.Time
		done := false
		for !done {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				done = true
			}
			events := recorder.Intervals(last, time.Time{})
			if len(events) > 0 {
				for _, event := range events {
					if !event.From.Equal(event.To) {
						continue
					}
					fmt.Fprintln(opt.Out, event.String())
				}
				last = events[len(events)-1].From
			}
		}
	}()

	<-ctx.Done()

	cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cleanupCancel()
	if err := m.Stop(cleanupContext); err != nil {
		fmt.Fprintf(os.Stderr, "error cleaning up, still reporting as best as possible: %v\n", err)
	}

	// Store events to artifact directory
	if len(opt.ArtifactDir) != 0 {
		if err := m.SerializeResults(ctx, "invariants", ""); err != nil {
			return err
		}
	}
	return nil
}
