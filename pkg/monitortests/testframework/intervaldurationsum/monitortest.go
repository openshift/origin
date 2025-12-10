package intervaldurationsum

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

// intervalDurationSum is a monitor test that sums the total duration of intervals
// matching specific sources and writes the results to an autodl file.
//
// The generated autodl file will have the following schema:
//   - IntervalSource (string): The source type of the intervals
//   - TotalDurationSeconds (float64): Sum of all interval durations in seconds for that source
//
// The autodl file will be named: interval_duration_sum{timeSuffix}-autodl.json
type intervalDurationSum struct {
	adminRESTConfig *rest.Config
}

// NewIntervalDurationSum creates a monitor test that sums the total duration of intervals
// for specific sources and writes the results to an autodl file.
func NewIntervalDurationSum() monitortestframework.MonitorTest {
	return &intervalDurationSum{}
}

func (w *intervalDurationSum) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *intervalDurationSum) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *intervalDurationSum) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (w *intervalDurationSum) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *intervalDurationSum) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *intervalDurationSum) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	logger := logrus.WithField("MonitorTest", "IntervalDurationSum")

	// Define the interval sources to track
	sourcesToTrack := []monitorapi.IntervalSource{
		monitorapi.SourceMetricsEndpointDown,
		monitorapi.SourceCPUMonitor,
	}

	// Calculate total duration for each source
	rows := []map[string]string{}
	for _, source := range sourcesToTrack {
		matchingIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Source == source
		})

		var totalDurationSeconds float64
		for _, interval := range matchingIntervals {
			duration := interval.To.Sub(interval.From).Seconds()
			totalDurationSeconds += duration
		}

		logger.Infof("Total duration for source %s: %.2f seconds across %d intervals", source, totalDurationSeconds, len(matchingIntervals))

		rows = append(rows, map[string]string{
			"IntervalSource":       string(source),
			"TotalDurationSeconds": fmt.Sprintf("%.2f", totalDurationSeconds),
		})
	}

	// Create autodl artifact with total durations per source
	dataFile := dataloader.DataFile{
		TableName: "interval_duration_sum",
		Schema: map[string]dataloader.DataType{
			"IntervalSource":       dataloader.DataTypeString,
			"TotalDurationSeconds": dataloader.DataTypeFloat64,
		},
		Rows: rows,
	}

	// Create the file name using the autodl suffix
	fileName := filepath.Join(storageDir, fmt.Sprintf("interval-duration-sum%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))

	// Write the data file
	err := dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logger.WithError(err).Warnf("unable to write data file: %s", fileName)
	}

	return nil
}

func (w *intervalDurationSum) Cleanup(ctx context.Context) error {
	return nil
}
