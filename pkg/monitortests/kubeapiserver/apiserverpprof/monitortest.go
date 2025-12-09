package apiserverpprof

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
)

type pprofSnapshot struct {
	when         time.Time
	duration     time.Duration
	responseCode int
	err          error
	filePath     string // path where file was written (if successful)
}

type apiserverPprofCollector struct {
	collectionDone chan struct{}
	snapshots      []pprofSnapshot
	artifactDir    string
}

func NewApiserverPprofCollector() monitortestframework.MonitorTest {
	return &apiserverPprofCollector{
		collectionDone: make(chan struct{}),
		snapshots:      make([]pprofSnapshot, 0),
	}
}

func (w *apiserverPprofCollector) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	// Get ARTIFACT_DIR early so we can write files during collection
	artifactDir := os.Getenv("ARTIFACT_DIR")
	if artifactDir == "" {
		return fmt.Errorf("ARTIFACT_DIR environment variable is not set")
	}

	// Create subdirectory for pprof files
	pprofDir := filepath.Join(artifactDir, "apiserver-pprof")
	if err := os.MkdirAll(pprofDir, 0755); err != nil {
		return fmt.Errorf("unable to create directory %s: %w", pprofDir, err)
	}

	w.artifactDir = pprofDir
	return nil
}

func collectPprofProfile(ch chan *pprofSnapshot, artifactDir string) {
	oc := exutil.NewCLIWithoutNamespace("apiserver-pprof").AsAdmin()
	now := time.Now()
	start := time.Now()

	cmd := oc.Run("get", "--raw", "/debug/pprof/profile?seconds=15")
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
		snapshot.responseCode = 200

		// Write file immediately to avoid holding in memory
		filename := fmt.Sprintf("apiserver-%s.pprof", now.Format("20060102-150405"))
		filePath := filepath.Join(artifactDir, filename)
		if writeErr := os.WriteFile(filePath, []byte(out), 0644); writeErr != nil {
			snapshot.err = fmt.Errorf("failed to write pprof file: %w", writeErr)
		} else {
			snapshot.filePath = filePath
		}
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
					fmt.Printf("apiserver pprof collected at %s (duration: %v, response code: %d, file: %s)\n",
						snap.when.Format(time.RFC3339), snap.duration, snap.responseCode, snap.filePath)
				}
			}
			w.collectionDone <- struct{}{}
		}()

		// Continuously collect pprof profiles (each collection takes ~15 seconds)
		// Start the next collection immediately after the previous one completes
		for {
			select {
			case <-ctx.Done():
				close(snapshots)
				return
			default:
				collectPprofProfile(snapshots, w.artifactDir)
			}
		}
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
	// Files were already written during collection, just report summary
	successCount := 0
	failureCount := 0

	for _, snap := range w.snapshots {
		if snap.err != nil {
			failureCount++
		} else if snap.filePath != "" {
			successCount++
		}
	}

	fmt.Printf("Apiserver pprof collection summary: %d profiles written, %d failures, stored in %s\n",
		successCount, failureCount, w.artifactDir)

	return nil
}

func (w *apiserverPprofCollector) Cleanup(ctx context.Context) error {
	// Context cancellation will stop the polling in StartCollection
	return nil
}
