package highcputestanalyzer

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

// highCPUTestAnalyzer looks for e2e tests that overlap with high CPU metric intervals and generates a data file with the results.
// The data file uses the autodl framework and thus is ingested automatically into bigquery, where we can then search
// for tests failures that are correlated with high CPU. (either failing because of it, or perhaps causing it)
type highCPUTestAnalyzer struct {
	adminRESTConfig *rest.Config
}

func NewHighCPUTestAnalyzer() monitortestframework.MonitorTest {
	return &highCPUTestAnalyzer{}
}

func (w *highCPUTestAnalyzer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *highCPUTestAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *highCPUTestAnalyzer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*highCPUTestAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*highCPUTestAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*highCPUTestAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	// Find E2E tests that overlap with high CPU alerts
	rows := findE2EIntervalsOverlappingHighCPU(finalIntervals)

	// Create data file with the collected rows
	dataFile := dataloader.DataFile{
		TableName: "high_cpu_e2e_tests",
		Schema: map[string]dataloader.DataType{
			"TestName": dataloader.DataTypeString,
			"Success":  dataloader.DataTypeInteger,
		},
		Rows: rows,
	}

	// Create the file name using the specified format
	fileName := filepath.Join(storageDir, fmt.Sprintf("high-cpu-e2etests%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))

	// Write the data file
	err := dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
	}

	return nil
}

func (*highCPUTestAnalyzer) Cleanup(ctx context.Context) error {
	return nil
}

// findE2EIntervalsOverlappingHighCPU finds E2E test intervals that overlap with high CPU metric intervals
// Each test is only reported once, even if it overlaps with multiple high CPU periods, unless the test executed twice. (retry)
func findE2EIntervalsOverlappingHighCPU(intervals monitorapi.Intervals) []map[string]string {
	// Filter for high CPU metric intervals collected by our monitor
	highCPUIntervals := intervals.Filter(func(interval monitorapi.Interval) bool {
		return interval.Source == monitorapi.SourceCPUMonitor &&
			interval.Message.Reason == monitorapi.IntervalReason("HighCPUUsage")
	})

	// Filter for E2E test intervals
	e2eTestIntervals := intervals.Filter(func(interval monitorapi.Interval) bool {
		return interval.Source == monitorapi.SourceE2ETest
	})

	// Find E2E tests that overlap with alert intervals
	// Use a map to track processed tests to ensure each test is only reported once
	processedTests := make(map[string]bool)
	rows := []map[string]string{}

	var highCPUSuccessfulTests, highCPUFailedTests int

	// Iterate through E2E tests first to ensure each test is only processed once
	for _, testInterval := range e2eTestIntervals {
		testName, exists := testInterval.Locator.Keys[monitorapi.LocatorE2ETestKey]
		if !exists {
			continue
		}

		// Check if this test overlaps with any high CPU interval
		overlapsWithHighCPU := false
		for _, highCPUInterval := range highCPUIntervals {
			if utility.IntervalsOverlap(highCPUInterval, testInterval) {
				overlapsWithHighCPU = true
				break
			}
		}

		if overlapsWithHighCPU {
			// Mark this test as processed
			processedTests[testName] = true

			// Determine success value based on status annotation
			success := "0"
			if status, exists := testInterval.Message.Annotations[monitorapi.AnnotationStatus]; exists && status == "Passed" {
				success = "1"
				highCPUSuccessfulTests++
			} else {
				highCPUFailedTests++
			}

			rows = append(rows, map[string]string{
				"TestName": testName,
				"Success":  success,
			})
		}
	}

	logrus.Infof("High CPU correlated tests: %d successful, %d failed", highCPUSuccessfulTests, highCPUFailedTests)

	return rows
}
