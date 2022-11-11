package monitor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Options is used to run a monitoring process against the provided server as
// a command line interaction.
type Options struct {
	Out, ErrOut io.Writer
	ArtifactDir string

	AdditionalEventIntervalRecorders []StartEventIntervalRecorderFunc
}

// Run starts monitoring the cluster by invoking Start, periodically printing the
// events accumulated to Out. When the user hits CTRL+C or signals termination the
// condition intervals (all non-instantaneous events) are reported to Out.
func (opt *Options) Run() error {
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

	restConfig, err := GetMonitorRESTConfig()
	if err != nil {
		return err
	}
	m, err := Start(ctx, restConfig, opt.AdditionalEventIntervalRecorders)
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
		recordedEvents := m.Intervals(time.Time{}, time.Time{})
		recordedResources := m.CurrentResourceState()
		timeSuffix := fmt.Sprintf("_%s", time.Now().UTC().Format("20060102-150405"))

		eventDir := fmt.Sprintf("%s/monitor-events", opt.ArtifactDir)
		if err := os.MkdirAll(eventDir, os.ModePerm); err != nil {
			fmt.Printf("Failed to create monitor-events directory, err: %v\n", err)
			return err
		}
		err := WriteEventsForJobRun(eventDir, recordedResources, recordedEvents, timeSuffix)
		if err != nil {
			fmt.Printf("Failed to write event data, err: %v\n", err)
			return err
		}
	}
	return nil
}
