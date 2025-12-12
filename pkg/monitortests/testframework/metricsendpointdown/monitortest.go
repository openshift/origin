package metricsendpointdown

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

const testName = "[sig-node] kubelet metrics endpoints should always be reachable"

type metricsEndpointDown struct {
	adminRESTConfig *rest.Config
}

func NewMetricsEndpointDown() monitortestframework.MonitorTest {
	return &metricsEndpointDown{}
}

func (w *metricsEndpointDown) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *metricsEndpointDown) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *metricsEndpointDown) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	intervals, err := buildIntervalsForMetricsEndpointsDown(ctx, w.adminRESTConfig, beginning)
	return intervals, nil, err
}

func (*metricsEndpointDown) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*metricsEndpointDown) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	failures := []string{}
	logger := logrus.WithField("MonitorTest", "MetricsEndpointDown")
	metricsEndpointDownIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceMetricsEndpointDown
	})
	logger.Infof("found %d metrics endpoint down intervals", len(metricsEndpointDownIntervals))

	// We know these endpoints go down both during node update, and obviously during reboot, ignore overlap
	// with either:
	nodeUpdateIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return (eventInterval.Source == monitorapi.SourceNodeState && eventInterval.Message.Annotations["phase"] == "Update") ||
			(eventInterval.Source == monitorapi.SourceNodeState && eventInterval.Message.Annotations["phase"] == "Reboot")
	})
	logger.Infof("found %d node update intervals", len(nodeUpdateIntervals))

	for _, downInterval := range metricsEndpointDownIntervals {
		logger.Infof("checking metrics down interval: %s", downInterval)
		restartsForNodeIntervals := nodeUpdateIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Locator.Keys[monitorapi.LocatorNodeKey] == downInterval.Locator.Keys[monitorapi.LocatorNodeKey]
		})
		overlapIntervals := utility.FindOverlap(restartsForNodeIntervals, downInterval)
		if len(overlapIntervals) == 0 {
			failures = append(failures, downInterval.String())
			logger.Info("found no overlap with a node update")
		} else {
			logger.Infof("found overlap with a node update: %s", overlapIntervals[0])
		}
	}
	junits := []*junitapi.JUnitTestCase{}
	if len(failures) > 0 {
		testOutput := fmt.Sprintf("found prometheus reporting metrics endpoints down outside of a node update: \n  %s",
			strings.Join(failures, "\n  "))
		// This metrics down interval did not overlap with any update for the corresponding node, fail/flake a junit:
		// Limit to kubelet service, all we're querying right now?
		junits = append(junits, &junitapi.JUnitTestCase{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: testOutput,
			},
		})
	}
	// Add a success so this is marked as a flake at worst, no idea what this will unleash in the wild.
	junits = append(junits, &junitapi.JUnitTestCase{
		Name: testName,
	})
	return junits, nil
}

func (*metricsEndpointDown) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	// Filter for metrics endpoint down intervals
	metricsEndpointDownIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceMetricsEndpointDown
	})

	// Calculate total outage time across all endpoints
	var totalOutageSeconds float64
	for _, interval := range metricsEndpointDownIntervals {
		duration := interval.To.Sub(interval.From).Seconds()
		totalOutageSeconds += duration
	}

	logger := logrus.WithField("MonitorTest", "MetricsEndpointDown")
	logger.Infof("Total metrics endpoint downtime: %.2f seconds across %d intervals", totalOutageSeconds, len(metricsEndpointDownIntervals))

	// Create autodl artifact with total outage time
	dataFile := dataloader.DataFile{
		TableName: "kubelet_metrics_endpoint_downtime",
		Schema: map[string]dataloader.DataType{
			"TotalOutageSeconds": dataloader.DataTypeFloat64,
		},
		Rows: []map[string]string{
			{
				"TotalOutageSeconds": fmt.Sprintf("%.2f", totalOutageSeconds),
			},
		},
	}

	// Create the file name using the autodl suffix
	fileName := filepath.Join(storageDir, fmt.Sprintf("kubelet-metrics-endpoint-downtime%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))

	// Write the data file
	err := dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logger.WithError(err).Warnf("unable to write data file: %s", fileName)
	}

	return nil
}

func (*metricsEndpointDown) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
