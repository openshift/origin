package highcputestanalyzer

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

// highCPUTestAnalyzer looks for e2e tests that overlap with high CPU alerts and generates a data file with the results.
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

// findE2EIntervalsOverlappingHighCPU finds E2E test intervals that overlap with high CPU alert intervals
func findE2EIntervalsOverlappingHighCPU(intervals monitorapi.Intervals) []map[string]string {
	// Filter for alert intervals of interest
	alertIntervals := intervals.Filter(func(interval monitorapi.Interval) bool {
		if interval.Source != monitorapi.SourceAlert {
			return false
		}

		alertName, exists := interval.Locator.Keys["alert"]
		return exists && (alertName == "ExtremelyHighIndividualControlPlaneCPU" || alertName == "HighOverallControlPlaneCPU")
	})

	// Filter for E2E test intervals
	e2eTestIntervals := intervals.Filter(func(interval monitorapi.Interval) bool {
		return interval.Source == monitorapi.SourceE2ETest
	})

	// Find E2E tests that overlap with alert intervals
	rows := []map[string]string{}

	var highCPUSuccessfulTests, highCPUFailedTests int
	for _, alertInterval := range alertIntervals {
		for _, testInterval := range e2eTestIntervals {
			// Check if test interval overlaps with alert interval
			if overlaps(alertInterval, testInterval) {
				testName, exists := testInterval.Locator.Keys[monitorapi.LocatorE2ETestKey]
				if !exists {
					continue
				}

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
	}

	logrus.Infof("High CPU correlated tests: %d successful, %d failed", highCPUSuccessfulTests, highCPUFailedTests)

	return rows
}

// overlaps checks if two intervals overlap in time
func overlaps(interval1, interval2 monitorapi.Interval) bool {
	// If either interval has a zero end time, treat it as ongoing to the end of time
	end1 := interval1.To
	if end1.IsZero() {
		end1 = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	}

	end2 := interval2.To
	if end2.IsZero() {
		end2 = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	}

	// Check for overlap
	return (interval1.From.Before(end2) || interval1.From.Equal(end2)) &&
		(interval2.From.Before(end1) || interval2.From.Equal(end1))
}
