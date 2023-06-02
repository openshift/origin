package monitor_command

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption/controlplane"
	"github.com/openshift/origin/test/extended/util/disruption/externalservice"
	"github.com/openshift/origin/test/extended/util/disruption/frontends"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
)

// RunMonitorOptions is used to run a monitoring process against the provided server as
// a command line interaction.
type RunMonitorOptions struct {
	Out, ErrOut io.Writer
	ArtifactDir string

	AdditionalEventIntervalRecorders []monitor.StartEventIntervalRecorderFunc

	TimelineOptions TimelineOptions
}

func NewRunMonitorOptions(ioStreams genericclioptions.IOStreams) *RunMonitorOptions {
	timelineOptions := NewTimelineOptions(ioStreams)

	return &RunMonitorOptions{
		Out:    ioStreams.Out,
		ErrOut: ioStreams.ErrOut,
		AdditionalEventIntervalRecorders: []monitor.StartEventIntervalRecorderFunc{
			controlplane.StartAllAPIMonitoring,
			frontends.StartAllIngressMonitoring,
			externalservice.StartExternalServiceMonitoring,
		},

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
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal, 2)
	go func() {
		<-abortCh
		fmt.Fprintf(opt.ErrOut, "Interrupted, terminating\n")
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

	restConfig, err := monitor.GetMonitorRESTConfig()
	if err != nil {
		return err
	}
	m, err := monitor.Start(ctx, restConfig, opt.AdditionalEventIntervalRecorders)
	if err != nil {
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
			events := m.Intervals(last, time.Time{})
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

	time.Sleep(150 * time.Millisecond)
	if events := m.Conditions(time.Time{}, time.Time{}); len(events) > 0 {
		fmt.Fprintf(opt.Out, "\nConditions:\n\n")
		for _, event := range events {
			fmt.Fprintln(opt.Out, event.String())
		}
	}

	// Store events to artifact directory
	if len(opt.ArtifactDir) != 0 {
		intervals := m.Intervals(time.Time{}, time.Time{})
		recordedResources := m.CurrentResourceState()
		timeSuffix := fmt.Sprintf("_%s", time.Now().UTC().Format("20060102-150405"))

		eventDir := fmt.Sprintf("%s/monitor-events", opt.ArtifactDir)
		if err := os.MkdirAll(eventDir, os.ModePerm); err != nil {
			fmt.Printf("Failed to create monitor-events directory, err: %v\n", err)
			return err
		}

		if err := monitor.WriteEventsForJobRun(eventDir, recordedResources, intervals, timeSuffix); err != nil {
			fmt.Printf("Failed to write event data, err: %v\n", err)
			return err
		}

		err = monitor.UploadIntervalsToLoki(intervals)
		if err != nil {
			// Best effort, we do not want to error out here:
			logrus.WithError(err).Warn("unable to upload intervals to loki")
		}

		if err := monitor.WriteTrackedResourcesForJobRun(eventDir, recordedResources, intervals, timeSuffix); err != nil {
			fmt.Printf("Failed to write resource data, err: %v\n", err)
			return err
		}
		// we know these names because the methods above use known file locations
		eventJSONFilename := filepath.Join(eventDir, fmt.Sprintf("e2e-events%s.json", timeSuffix))
		podResourceFilename := filepath.Join(eventDir, fmt.Sprintf("resource-pods%s.json", timeSuffix))

		// override specifics for our use-case
		t := opt.TimelineOptions
		timelineOptions := &t
		timelineOptions.MonitorEventFilename = eventJSONFilename
		timelineOptions.PodResourceFilename = podResourceFilename
		timelineOptions.TimelineType = "everything"
		timelineOptions.OutputType = "html"
		if err := t.Complete(); err != nil {
			fmt.Printf("Failed to complete timeline options, err: %v\n", err)
			return err
		}
		if err := t.Validate(); err != nil {
			fmt.Printf("Failed to validate timeline options, err: %v\n", err)
			return err
		}
		if err := t.ToTimeline().Run(); err != nil {
			fmt.Printf("Failed to run timeline, err: %v\n", err)
			return err
		}
	}
	return nil
}
