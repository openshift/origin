package monitortest

import (
	"context"
	"errors"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/auditloganalyzer"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/auditloganalyzer/eventsprovider"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/auditloganalyzer/summary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// NewAuditLogAnalyzer returns a cluster monitoring test that analyzes the
// cluster audit log events.
func NewAuditLogAnalyzer() monitortestframework.MonitorTest {
	return &clusterMonitor{
		collectors: []auditloganalyzer.AuditEventCollector{
			summary.NewAnalyzer(),
		},
	}
}

type clusterMonitor struct {
	config     *rest.Config
	collectors []auditloganalyzer.AuditEventCollector
}

func (m *clusterMonitor) StartCollection(ctx context.Context, config *rest.Config, recorder monitorapi.RecorderWriter) error {
	m.config = config
	var errs []error
	for _, collector := range m.collectors {
		if startable, ok := collector.(auditloganalyzer.Startable); ok {
			err := startable.StartClusterMonitoring(ctx, config, recorder)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *clusterMonitor) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	client, err := kubernetes.NewForConfig(m.config)
	if err != nil {
		return nil, nil, err
	}

	var errs []error

	// first, process the audit events
	eventsProvider := eventsprovider.NewClusterEvents(&beginning, &end)
	collectors := auditloganalyzer.AuditEventCollectors(m.collectors)
	for e := range eventsProvider.Events(ctx, client) {
		collectors.Collect(e)
	}
	errs = append(errs, eventsProvider.Err())

	// collect points of interest generated from processing the audit log events
	var rawIntervals []monitorapi.Interval
	var tests []*junitapi.JUnitTestCase
	for _, collector := range collectors {
		if inspector, ok := collector.(auditloganalyzer.TestDataCollector); ok {
			intervals, units, err := inspector.InspectTestArtifacts(ctx, storageDir, beginning, end)
			rawIntervals = append(rawIntervals, []monitorapi.Interval(intervals)...)
			tests = append(tests, units...)
			errs = append(errs, err)
		}
		if inspector, ok := collector.(auditloganalyzer.ClusterDataCollector); ok {
			intervals, units, err := inspector.InspectCluster(ctx, beginning, end)
			rawIntervals = append(rawIntervals, []monitorapi.Interval(intervals)...)
			tests = append(tests, units...)
			errs = append(errs, err)
		}
	}
	return rawIntervals, tests, errors.Join(errs...)
}

func (m *clusterMonitor) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	var computedIntervals []monitorapi.Interval
	var errs []error
	for _, collector := range m.collectors {
		if computer, ok := collector.(auditloganalyzer.IntervalComputer); ok {
			intervals, err := computer.ProcessAggregatedIntervals(ctx, startingIntervals, recordedResources, beginning, end)
			computedIntervals = append(computedIntervals, []monitorapi.Interval(intervals)...)
			errs = append(errs, err)
		}
	}
	return computedIntervals, errors.Join(errs...)
}

func (m *clusterMonitor) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	var tests []*junitapi.JUnitTestCase
	var errs []error
	for _, collector := range m.collectors {
		if computer, ok := collector.(auditloganalyzer.TestEvaluator); ok {
			units, err := computer.Evaluate(ctx, finalIntervals)
			tests = append(tests, units...)
			errs = append(errs, err)
		}
	}
	return tests, errors.Join(errs...)
}

func (m *clusterMonitor) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	var errs []error
	for _, collector := range m.collectors {
		if computer, ok := collector.(auditloganalyzer.StorageContentWriter); ok {
			err := computer.SaveArtifacts(ctx, storageDir, timeSuffix, finalIntervals, finalResourceState)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *clusterMonitor) Cleanup(ctx context.Context) error {
	var errs []error
	for _, collector := range m.collectors {
		if computer, ok := collector.(auditloganalyzer.Stoppable); ok {
			err := computer.StopClusterMonitoring(ctx)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
