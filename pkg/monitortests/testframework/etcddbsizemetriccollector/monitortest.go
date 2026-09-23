package etcddbsizemetriccollector

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

type etcdDBSizeMetricCollector struct {
	adminRESTConfig *rest.Config
	growthThreshold float64 // MB growth in 2 minutes
}

func NewEtcdDBSizeMetricCollector() monitortestframework.MonitorTest {
	return &etcdDBSizeMetricCollector{
		growthThreshold: 25.0, // Default to 25MB growth in 2 minutes
	}
}

func (w *etcdDBSizeMetricCollector) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *etcdDBSizeMetricCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *etcdDBSizeMetricCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	logger := logrus.WithField("MonitorTest", "EtcdDBSizeMetricCollector")

	intervals, err := w.buildIntervalsForDBGrowth(ctx, w.adminRESTConfig, beginning)
	if err != nil {
		return nil, nil, err
	}

	logger.Infof("collected %d etcd DB size growth intervals", len(intervals))
	return intervals, nil, nil
}

func (w *etcdDBSizeMetricCollector) buildIntervalsForDBGrowth(ctx context.Context, restConfig *rest.Config, startTime time.Time) ([]monitorapi.Interval, error) {
	logger := logrus.WithField("func", "buildIntervalsForDBGrowth")
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

	if intervals, err := prometheus.EnsureThanosQueriersConnectedToPromSidecars(ctx, prometheusClient); err != nil {
		return intervals, err
	}

	timeRange := prometheusv1.Range{
		Start: startTime,
		End:   time.Now(),
		Step:  30 * time.Second, // Sample every 30 seconds for better granularity
	}

	// Query for etcd DB size growth in MB per 2 minutes, only where growth exceeds threshold
	dbGrowthQuery := `delta(etcd_mvcc_db_total_size_in_bytes[2m]) / 1024 / 1024 > 25`
	dbGrowthMetrics, warningsForQuery, err := prometheusClient.QueryRange(ctx, dbGrowthQuery, timeRange)
	if err != nil {
		return nil, err
	}
	if len(warningsForQuery) > 0 {
		for _, w := range warningsForQuery {
			logger.Warnf("etcd DB growth metric query warning: %s", w)
		}
	}

	return w.createIntervalsFromDBGrowthMetrics(logger, dbGrowthMetrics)
}

func (w *etcdDBSizeMetricCollector) createIntervalsFromDBGrowthMetrics(logger logrus.FieldLogger, promVal prometheustypes.Value) ([]monitorapi.Interval, error) {
	ret := []monitorapi.Interval{}

	switch {
	case promVal.Type() == prometheustypes.ValMatrix:
		promMatrix := promVal.(prometheustypes.Matrix)
		for _, promSampleStream := range promMatrix {
			instance := string(promSampleStream.Metric["instance"])
			pod := string(promSampleStream.Metric["pod"])
			ns := string(promSampleStream.Metric["namespace"])

			// Create locator for the pod - etcd pods are typically in openshift-etcd namespace
			// Use pod/namespace if available, otherwise fall back to instance
			var locator monitorapi.Locator
			if pod != "" && ns != "" {
				locator = monitorapi.NewLocator().PodFromNames(ns, pod, "")
			} else {
				locator = monitorapi.NewLocator().PodFromNames("", instance, "")
			}

			// Track consecutive high growth periods
			var highGrowthStart *time.Time
			var highGrowthEnd *time.Time
			var peakGrowth float64

			// Determine label to use for human-readable messages
			label := pod
			if label == "" {
				label = instance
			}

			for _, currValue := range promSampleStream.Values {
				currTime := currValue.Timestamp.Time()
				growthMB := float64(currValue.Value)

				// Check if DB growth exceeds threshold
				if growthMB > w.growthThreshold {
					// If not currently in a high growth period, start a new one
					if highGrowthStart == nil {
						highGrowthStart = &currTime
						peakGrowth = growthMB
					} else {
						// Continue the current high growth period, track peak growth
						if growthMB > peakGrowth {
							peakGrowth = growthMB
						}
					}
					// Always update the end time to current time for continuous high growth
					highGrowthEnd = &currTime
				} else {
					// DB growth dropped below threshold
					if highGrowthStart != nil && highGrowthEnd != nil {
						// Create interval for the high growth period that just ended
						ret = append(ret, w.createDBGrowthInterval(locator, label, *highGrowthStart, *highGrowthEnd, peakGrowth))
						// Reset tracking variables
						highGrowthStart = nil
						highGrowthEnd = nil
						peakGrowth = 0
					}
				}
			}

			// Handle case where high growth period extends to the end of the monitoring window
			if highGrowthStart != nil && highGrowthEnd != nil {
				ret = append(ret, w.createDBGrowthInterval(locator, label, *highGrowthStart, *highGrowthEnd, peakGrowth))
			}
		}

	default:
		logger.WithField("type", promVal.Type()).Warning("unhandled prometheus value type received")
	}

	return ret, nil
}

func (w *etcdDBSizeMetricCollector) createDBGrowthInterval(locator monitorapi.Locator, podOrInstance string, start, end time.Time, growthMB float64) monitorapi.Interval {
	// Create message with all necessary information
	msgBuilder := monitorapi.NewMessage().
		Reason(monitorapi.IntervalReason("EtcdDBRapidGrowth")).
		HumanMessage(fmt.Sprintf("etcd DB growth above %.1fMB threshold on pod %s", w.growthThreshold, podOrInstance)).
		WithAnnotation("growth_threshold_mb", fmt.Sprintf("%.1f", w.growthThreshold))

	if growthMB > 0 {
		msgBuilder = msgBuilder.WithAnnotation("peak_growth_mb", fmt.Sprintf("%.2f", growthMB))
	}

	// Create and build the interval directly
	interval := monitorapi.NewInterval(monitorapi.SourceEtcdDBSizeMonitor, monitorapi.Warning).
		Locator(locator).
		Message(msgBuilder).
		Display()

	return interval.Build(start, end)
}

func (*etcdDBSizeMetricCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *etcdDBSizeMetricCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	// This monitor test is purely for data collection, not for generating test cases
	return nil, nil
}

func (*etcdDBSizeMetricCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*etcdDBSizeMetricCollector) Cleanup(ctx context.Context) error {
	return nil
}
