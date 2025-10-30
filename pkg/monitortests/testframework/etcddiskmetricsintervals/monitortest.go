package etcddiskmetricsintervals

import (
	"context"
	"fmt"
	"time"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/library-go/test/library/metrics"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/prometheus"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prometheustypes "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type etcdDiskMetricsCollector struct {
	adminRESTConfig         *rest.Config
	commitDurationThreshold float64
	walFsyncThreshold       float64
}

func NewEtcdDiskMetricsCollector() monitortestframework.MonitorTest {
	return &etcdDiskMetricsCollector{
		commitDurationThreshold: 0.025, // 25ms threshold, defined upstream
		walFsyncThreshold:       0.01,  // 10ms threshold, defined upstream
	}
}

func (w *etcdDiskMetricsCollector) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *etcdDiskMetricsCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *etcdDiskMetricsCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	logger := logrus.WithField("MonitorTest", "EtcdDiskMetricsCollector")

	intervals, err := w.buildIntervalsForEtcdDiskMetrics(ctx, w.adminRESTConfig, beginning)
	if err != nil {
		return nil, nil, err
	}

	logger.Infof("collected %d etcd disk metrics intervals", len(intervals))
	return intervals, nil, nil
}

func (w *etcdDiskMetricsCollector) buildIntervalsForEtcdDiskMetrics(ctx context.Context, restConfig *rest.Config, startTime time.Time) ([]monitorapi.Interval, error) {
	logger := logrus.WithField("func", "buildIntervalsForEtcdDiskMetrics")
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	routeClient, err := routeclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	_, err = kubeClient.CoreV1().Namespaces().Get(ctx, "openshift-monitoring", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return []monitorapi.Interval{}, nil
	} else if err != nil {
		return nil, err
	}

	prometheusClient, err := metrics.NewPrometheusClient(ctx, kubeClient, routeClient)
	if err != nil {
		return nil, err
	}

	if _, err := prometheus.EnsureThanosQueriersConnectedToPromSidecars(ctx, prometheusClient); err != nil {
		return nil, err
	}

	timeRange := prometheusv1.Range{
		Start: startTime,
		End:   time.Now(),
		Step:  30 * time.Second, // Sample every 30 seconds for better granularity
	}

	var allIntervals []monitorapi.Interval

	// Query for etcd disk backend commit duration over upstream guidance
	commitDurationQuery := `histogram_quantile(0.99, rate(etcd_disk_backend_commit_duration_seconds_bucket{job=~".*etcd.*"}[5m]))`
	commitMetrics, warningsForCommit, err := prometheusClient.QueryRange(ctx, commitDurationQuery, timeRange)
	if err != nil {
		return nil, err
	}
	if len(warningsForCommit) > 0 {
		for _, w := range warningsForCommit {
			logger.Warnf("Commit duration metric query warning: %s", w)
		}
	}

	commitIntervals, err := w.createIntervalsFromMetrics(logger, commitMetrics, monitorapi.SourceEtcdDiskCommitDuration, w.commitDurationThreshold, "disk backend commit duration")
	if err != nil {
		return nil, err
	}
	allIntervals = append(allIntervals, commitIntervals...)

	// Query for etcd disk WAL fsync duration over upstream guidance
	walFsyncQuery := `histogram_quantile(0.99, rate(etcd_disk_wal_fsync_duration_seconds_bucket{job=~".*etcd.*"}[5m]))`
	walFsyncMetrics, warningsForWal, err := prometheusClient.QueryRange(ctx, walFsyncQuery, timeRange)
	if err != nil {
		return nil, err
	}
	if len(warningsForWal) > 0 {
		for _, w := range warningsForWal {
			logger.Warnf("WAL fsync metric query warning: %s", w)
		}
	}

	walFsyncIntervals, err := w.createIntervalsFromMetrics(logger, walFsyncMetrics, monitorapi.SourceEtcdDiskWalFsyncDuration, w.walFsyncThreshold, "disk WAL fsync duration")
	if err != nil {
		return nil, err
	}
	allIntervals = append(allIntervals, walFsyncIntervals...)

	return allIntervals, nil
}

func (w *etcdDiskMetricsCollector) createIntervalsFromMetrics(logger logrus.FieldLogger, promVal prometheustypes.Value, source monitorapi.IntervalSource, threshold float64, metricType string) ([]monitorapi.Interval, error) {
	ret := []monitorapi.Interval{}

	switch {
	case promVal.Type() == prometheustypes.ValMatrix:
		promMatrix := promVal.(prometheustypes.Matrix)
		for _, promSampleStream := range promMatrix {
			pod := string(promSampleStream.Metric["pod"])
			ns := string(promSampleStream.Metric["namespace"])

			// Create locator for the pod - etcd pods are typically in openshift-etcd namespace
			// but we'll use empty namespace and uid as we don't have them from the metrics
			locator := monitorapi.NewLocator().PodFromNames(ns, pod, "")

			// Track consecutive high duration periods
			var highDurationStart *time.Time
			var highDurationEnd *time.Time
			var peakDuration float64

			for _, currValue := range promSampleStream.Values {
				currTime := currValue.Timestamp.Time()
				duration := float64(currValue.Value)

				// Check if duration exceeds threshold
				if duration > threshold {
					// If not currently in a high duration period, start a new one
					if highDurationStart == nil {
						highDurationStart = &currTime
						peakDuration = duration
					} else {
						// Continue the current high duration period, track peak duration
						if duration > peakDuration {
							peakDuration = duration
						}
					}
					// Always update the end time to current time for continuous high duration
					highDurationEnd = &currTime
				} else {
					// Duration dropped below threshold
					if highDurationStart != nil && highDurationEnd != nil {
						// Create interval for the high duration period that just ended
						ret = append(ret, w.createDiskMetricInterval(locator, pod, *highDurationStart, *highDurationEnd, peakDuration, source, threshold, metricType))
						// Reset tracking variables
						highDurationStart = nil
						highDurationEnd = nil
						peakDuration = 0
					}
				}
			}

			// Handle case where high duration period extends to the end of the monitoring window
			if highDurationStart != nil && highDurationEnd != nil {
				ret = append(ret, w.createDiskMetricInterval(locator, pod, *highDurationStart, *highDurationEnd, peakDuration, source, threshold, metricType))
			}
		}

	default:
		logger.WithField("type", promVal.Type()).Warning("unhandled prometheus value type received")
	}

	return ret, nil
}

func (w *etcdDiskMetricsCollector) createDiskMetricInterval(locator monitorapi.Locator, pod string, start, end time.Time, peakDuration float64, source monitorapi.IntervalSource, threshold float64, metricType string) monitorapi.Interval {
	// Create message with all necessary information
	msgBuilder := monitorapi.NewMessage().
		Reason(monitorapi.IntervalReason("HighEtcdDiskDuration")).
		HumanMessage(fmt.Sprintf("Etcd %s above upstream recommended %.3fs threshold on pod %s", metricType, threshold, pod)).
		WithAnnotation("duration_threshold", fmt.Sprintf("%.3f", threshold))

	if peakDuration > 0 {
		msgBuilder = msgBuilder.WithAnnotation("peak_duration", fmt.Sprintf("%.6f", peakDuration))
	}

	// Create and build the interval directly with the appropriate source
	interval := monitorapi.NewInterval(source, monitorapi.Warning).
		Locator(locator).
		Message(msgBuilder).
		Display()

	return interval.Build(start, end)
}

func (*etcdDiskMetricsCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *etcdDiskMetricsCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	// This monitor test is purely for data collection, not for generating test cases
	return nil, nil
}

func (*etcdDiskMetricsCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*etcdDiskMetricsCollector) Cleanup(ctx context.Context) error {
	return nil
}
