package apiserverpprof

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
)

type pprofSnapshot struct {
	when         time.Time
	data         []byte
	duration     time.Duration
	responseCode int
	err          error
}

const PollInterval = 15 * time.Second

type apiserverPprofCollector struct {
	collectionDone chan struct{}
	snapshots      []pprofSnapshot
}

func NewApiserverPprofCollector() monitortestframework.MonitorTest {
	return &apiserverPprofCollector{
		collectionDone: make(chan struct{}),
		snapshots:      make([]pprofSnapshot, 0),
	}
}

func (w *apiserverPprofCollector) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func collectPprofProfile(ch chan *pprofSnapshot) {
	oc := exutil.NewCLIWithoutNamespace("apiserver-pprof").AsAdmin()
	now := time.Now()
	start := time.Now()

	cmd := oc.Run("get", "--raw", "/debug/pprof/profile?seconds=10")
	out, err := cmd.Output()
	duration := time.Since(start)

	snapshot := &pprofSnapshot{
		when:     now,
		duration: duration,
		err:      err,
	}

	if err != nil {
		// Log error but continue - we'll record this failure
		snapshot.responseCode = 500 // Default error code
	} else {
		snapshot.data = []byte(out)
		snapshot.responseCode = 200
	}

	ch <- snapshot
}

func (w *apiserverPprofCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	go func(ctx context.Context) {
		snapshots := make(chan *pprofSnapshot)
		go func() {
			for snap := range snapshots {
				w.snapshots = append(w.snapshots, *snap)
				if snap.err != nil {
					fmt.Printf("apiserver pprof collection failed at %s (duration: %v): %v\n",
						snap.when.Format(time.RFC3339), snap.duration, snap.err)
				} else {
					fmt.Printf("apiserver pprof collected at %s (duration: %v, size: %d bytes, response code: %d)\n",
						snap.when.Format(time.RFC3339), snap.duration, len(snap.data), snap.responseCode)
				}
			}
			w.collectionDone <- struct{}{}
		}()

		// Poll every 15 seconds
		wait.UntilWithContext(ctx, func(ctx context.Context) { collectPprofProfile(snapshots) }, PollInterval)
		close(snapshots)
	}(ctx)

	return nil
}

func (w *apiserverPprofCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// Wait for collection goroutines to finish
	<-w.collectionDone

	return nil, nil, nil
}

func (w *apiserverPprofCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *apiserverPprofCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *apiserverPprofCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	artifactDir := os.Getenv("ARTIFACT_DIR")
	if artifactDir == "" {
		// Fall back to storageDir if ARTIFACT_DIR is not set
		artifactDir = storageDir
	}

	if artifactDir == "" {
		return fmt.Errorf("no storage directory available (ARTIFACT_DIR not set and storageDir is empty)")
	}

	// Create subdirectory for pprof files
	pprofDir := filepath.Join(artifactDir, "apiserver-pprof")
	if err := os.MkdirAll(pprofDir, 0755); err != nil {
		return fmt.Errorf("unable to create directory %s: %w", pprofDir, err)
	}

	var errors []error
	successCount := 0
	for _, snap := range w.snapshots {
		// Only write successful snapshots
		if snap.err == nil && len(snap.data) > 0 {
			filename := fmt.Sprintf("apiserver-%s.pprof", snap.when.Format("20060102-150405"))
			filepath := filepath.Join(pprofDir, filename)
			if err := os.WriteFile(filepath, snap.data, 0644); err != nil {
				errors = append(errors, fmt.Errorf("failed to write %s: %w", filepath, err))
			} else {
				successCount++
			}
		}
	}

	fmt.Printf("Wrote %d apiserver pprof profiles to %s\n", successCount, pprofDir)

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors writing pprof files: %v", len(errors), errors)
	}

	return nil
}

func (w *apiserverPprofCollector) Cleanup(ctx context.Context) error {
	// Context cancellation will stop the polling in StartCollection
	return nil
}
