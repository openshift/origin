package operator

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/openshift/origin/pkg/resourcewatch/git"
	"github.com/openshift/origin/pkg/resourcewatch/json"
	"github.com/openshift/origin/pkg/resourcewatch/observe"
	"k8s.io/klog/v2"
)

// this doesn't appear to handle restarts cleanly.  To do so it would need to compare the resource version that it is applying
// to the resource version present and it would need to handle unobserved deletions properly.  both are possible, neither is easy.
func RunResourceWatch(toJsonPath, fromJsonPath string) error {
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	log := klog.FromContext(ctx)

	abortCh := make(chan os.Signal, 2)
	go func() {
		<-abortCh
		klog.Errorf("Interrupted, terminating")
		cancelFn()
		sig := <-abortCh
		klog.Errorf("Interrupted twice, exiting (%s)\n", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	var source observe.ObservationSource
	var sink observe.ObservationSink

	if fromJsonPath != "" {
		file, err := os.Open(fromJsonPath)
		if err != nil {
			return fmt.Errorf("failed to open json file %q: %w", fromJsonPath, err)
		}

		source, err = json.Source(file)
		if err != nil {
			return err
		}
	} else {
		var err error
		source, err = observe.Source(log)
		if err != nil {
			return err
		}
	}

	if toJsonPath != "" {
		file, err := os.OpenFile(toJsonPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY|os.O_APPEND, 0664)
		if err != nil {
			return fmt.Errorf("failed to create json file %q: %w", toJsonPath, err)
		}

		sink, err = json.Sink(file)
		if err != nil {
			return err
		}
	} else {
		var err error
		sink, err = git.Sink(log)
		if err != nil {
			return err
		}
	}

	// Observers emit observations to this channel. We use this channel as a buffer between the observers and the git writer.
	// Memory consumption will grow if we can't write quickly enough.
	// For reference, the captured test data from a 4.20 installation has ~17000 observations.
	resourceC := make(chan *observe.ResourceObservation, 10000)

	sourceFinished := source(ctx, log, resourceC)
	sinkFinished := sink(ctx, log, resourceC)

	// Wait for the source and sink to finish.
	select {
	case <-sourceFinished:
		// The source finished. This will also happen if the context is cancelled.
		close(resourceC)
		log.Info("Source finished")

		// Wait for the sink to finish writing queued observations.
		<-sinkFinished

	case <-sinkFinished:
		// The sink exited. We're no longer writing data, so no point cleaning up
	}

	return nil
}
