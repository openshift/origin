package pertestintervalserializer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

var (
	_ monitortestframework.MonitorTest = (*perTestIntervalSerializer)(nil)
)

// reportTestName is stable across runs (no embedded numbers), as required for monitor JUnit names.
const reportTestName = "[sig-arch] per-test interval attribution report should be generated"

// perTestIntervalSerializer is an informational monitor test. It derives everything it needs from
// the final interval set, so it needs no cluster access and never fails the job; its value is the
// per-test-intervals JSON artifact it writes for offline analysis.
type perTestIntervalSerializer struct{}

func NewPerTestIntervalSerializer() monitortestframework.MonitorTest {
	return &perTestIntervalSerializer{}
}

func (w *perTestIntervalSerializer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *perTestIntervalSerializer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *perTestIntervalSerializer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*perTestIntervalSerializer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

// EvaluateTestsFromConstructedIntervals emits a single passing, informational JUnit case so the run
// is discoverable in CI. It never returns a FailureOutput, so it cannot fail the job.
func (*perTestIntervalSerializer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	report := buildReport(finalIntervals, time.Now())
	summary := fmt.Sprintf("generated per-test interval attribution for %d test windows", len(report.TestWindows))
	return []*junitapi.JUnitTestCase{{Name: reportTestName, SystemOut: summary}}, nil
}

func (w *perTestIntervalSerializer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	report := buildReport(finalIntervals, time.Now())
	jsonContent, err := json.MarshalIndent(report, "", "    ")
	if err != nil {
		return err
	}
	filename := filepath.Join(storageDir, fmt.Sprintf("per-test-intervals%s.json", timeSuffix))
	return os.WriteFile(filename, jsonContent, 0644)
}

func (*perTestIntervalSerializer) Cleanup(ctx context.Context) error {
	return nil
}
