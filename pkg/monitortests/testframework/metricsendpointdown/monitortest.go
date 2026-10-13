package metricsendpointdown

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	// Don't return intervals here - we'll filter them in ConstructComputedIntervals
	return nil, nil, nil
}

func (w *metricsEndpointDown) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	logger := logrus.WithField("MonitorTest", "MetricsEndpointDown")

	// Query Prometheus for metrics endpoint down intervals
	metricsEndpointDownIntervals, err := buildIntervalsForMetricsEndpointsDown(ctx, w.adminRESTConfig, beginning)
	if err != nil {
		return nil, err
	}
	logger.Infof("found %d metrics endpoint down intervals from Prometheus", len(metricsEndpointDownIntervals))

	// Filter for node update and reboot intervals
	nodeUpdateIntervals := startingIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return (eventInterval.Source == monitorapi.SourceNodeState && eventInterval.Message.Annotations["phase"] == "Update") ||
			(eventInterval.Source == monitorapi.SourceNodeState && eventInterval.Message.Annotations["phase"] == "Reboot")
	})
	logger.Infof("found %d node update/reboot intervals", len(nodeUpdateIntervals))

	// Filter out metrics endpoint down intervals that overlap with node updates/reboots
	filteredIntervals := monitorapi.Intervals{}
	for _, downInterval := range metricsEndpointDownIntervals {
		restartsForNodeIntervals := nodeUpdateIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
			return eventInterval.Locator.Keys[monitorapi.LocatorNodeKey] == downInterval.Locator.Keys[monitorapi.LocatorNodeKey]
		})
		overlapIntervals := utility.FindOverlap(restartsForNodeIntervals, downInterval)
		if len(overlapIntervals) == 0 {
			// No overlap with node update/reboot - keep this interval
			filteredIntervals = append(filteredIntervals, downInterval)
		} else {
			logger.Infof("filtering out metrics endpoint down interval due to overlap with node update/reboot: %s", downInterval)
		}
	}
	logger.Infof("returning %d filtered metrics endpoint down intervals (filtered out %d that overlapped with node updates/reboots)",
		len(filteredIntervals), len(metricsEndpointDownIntervals)-len(filteredIntervals))

	return filteredIntervals, nil
}

func (*metricsEndpointDown) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	logger := logrus.WithField("MonitorTest", "MetricsEndpointDown")

	// Get metrics endpoint down intervals - these have already been filtered in ConstructComputedIntervals
	// to exclude overlaps with node updates/reboots
	metricsEndpointDownIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceMetricsEndpointDown
	})
	logger.Infof("evaluating %d metrics endpoint down intervals (already filtered)", len(metricsEndpointDownIntervals))

	junits := []*junitapi.JUnitTestCase{}
	if len(metricsEndpointDownIntervals) > 0 {
		failures := []string{}
		for _, downInterval := range metricsEndpointDownIntervals {
			failures = append(failures, downInterval.String())
		}
		testOutput := fmt.Sprintf("found prometheus reporting metrics endpoints down outside of a node update: \n  %s",
			strings.Join(failures, "\n  "))
		junits = append(junits, &junitapi.JUnitTestCase{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: testOutput,
			},
		})
	}
	// Add a success so this is marked as a flake at worst
	junits = append(junits, &junitapi.JUnitTestCase{
		Name: testName,
	})
	return junits, nil
}

func (w *metricsEndpointDown) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	// No longer writing autodl files here - intervaldurationsum monitor test handles this
	return nil
}

func (*metricsEndpointDown) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
