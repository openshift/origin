package highcpumetriccollector

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

type highCPUMetricCollector struct {
	adminRESTConfig *rest.Config
	cpuThreshold    float64
}

func NewHighCPUMetricCollector() monitortestframework.MonitorTest {
	return &highCPUMetricCollector{
		cpuThreshold: 5.0, // Default to 95% threshold
	}
}

func (w *highCPUMetricCollector) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *highCPUMetricCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *highCPUMetricCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	intervals, err := w.buildIntervalsForHighCPU(ctx, w.adminRESTConfig, beginning)
	return intervals, nil, err
}

func (w *highCPUMetricCollector) buildIntervalsForHighCPU(ctx context.Context, restConfig *rest.Config, startTime time.Time) ([]monitorapi.Interval, error) {
	logger := logrus.WithField("func", "buildIntervalsForHighCPU")
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
	}

	prometheusClient, err := metrics.NewPrometheusClient(ctx, kubeClient, routeClient)
	if err != nil {
		return nil, err
	}

	intervals, err := prometheus.EnsureThanosQueriersConnectedToPromSidecars(ctx, prometheusClient)
	if err != nil {
		return intervals, err
	}

	timeRange := prometheusv1.Range{
		Start: startTime,
		End:   time.Now(),
		Step:  30 * time.Second, // Sample every 30 seconds for better granularity
	}

	// Query for CPU usage percentage per instance
	cpuQuery := `100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[1m])) * 100)`
	cpuMetrics, warningsForQuery, err := prometheusClient.QueryRange(ctx, cpuQuery, timeRange)
	if err != nil {
		return nil, err
	}
	if len(warningsForQuery) > 0 {
		for _, w := range warningsForQuery {
			logger.Warnf("CPU metric query warning: %s", w)
		}
	}

	return w.createIntervalsFromCPUMetrics(logger, cpuMetrics)
}

func (w *highCPUMetricCollector) createIntervalsFromCPUMetrics(logger logrus.FieldLogger, promVal prometheustypes.Value) ([]monitorapi.Interval, error) {
	ret := []monitorapi.Interval{}

	switch {
	case promVal.Type() == prometheustypes.ValMatrix:
		promMatrix := promVal.(prometheustypes.Matrix)
		for _, promSampleStream := range promMatrix {
			instance := string(promSampleStream.Metric["instance"])

			// Create locator for the node
			lb := monitorapi.NewLocator().NodeFromName(instance)

			msg := monitorapi.NewMessage().
				Reason(monitorapi.IntervalReason("HighCPUUsage")).
				HumanMessage(fmt.Sprintf("CPU usage above %.1f%% threshold on instance %s", w.cpuThreshold, instance)).
				WithAnnotation("cpu_threshold", fmt.Sprintf("%.1f", w.cpuThreshold))

			intervalTmpl := monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Warning).
				Locator(lb).
				Message(msg).
				Display()

			// Track consecutive high CPU periods
			var highCPUStart *time.Time
			var highCPUEnd *time.Time
			var peakCPUUsage float64

			for _, currValue := range promSampleStream.Values {
				currTime := currValue.Timestamp.Time()
				cpuUsage := float64(currValue.Value)

				// Check if CPU usage exceeds threshold
				if cpuUsage > w.cpuThreshold {
					// If not currently in a high CPU period, start a new one
					if highCPUStart == nil {
						highCPUStart = &currTime
						peakCPUUsage = cpuUsage
					} else {
						// Continue the current high CPU period, track peak usage
						if cpuUsage > peakCPUUsage {
							peakCPUUsage = cpuUsage
						}
					}
					// Always update the end time to current time for continuous high CPU
					highCPUEnd = &currTime
				} else {
					// CPU usage dropped below threshold
					if highCPUStart != nil && highCPUEnd != nil {
						// Create interval for the high CPU period that just ended
						ret = append(ret, w.createCPUInterval(*intervalTmpl, *highCPUStart, *highCPUEnd, peakCPUUsage))
						// Reset tracking variables
						highCPUStart = nil
						highCPUEnd = nil
						peakCPUUsage = 0
					}
				}
			}

			// Handle case where high CPU period extends to the end of the monitoring window
			if highCPUStart != nil && highCPUEnd != nil {
				ret = append(ret, w.createCPUInterval(*intervalTmpl, *highCPUStart, *highCPUEnd, peakCPUUsage))
			}
		}

	default:
		logger.WithField("type", promVal.Type()).Warning("unhandled prometheus value type received")
	}

	return ret, nil
}

func (w *highCPUMetricCollector) createCPUInterval(intervalTmpl monitorapi.IntervalBuilder, start, end time.Time, cpuUsage float64) monitorapi.Interval {
	// Create a new message with the peak usage annotation
	msg := intervalTmpl.BuildCondition().Message
	msgBuilder := monitorapi.NewMessage().
		Reason(msg.Reason).
		HumanMessage(msg.HumanMessage).
		WithAnnotation("cpu_threshold", fmt.Sprintf("%.1f", w.cpuThreshold))

	if cpuUsage > 0 {
		msgBuilder = msgBuilder.WithAnnotation("peak_cpu_usage", fmt.Sprintf("%.2f", cpuUsage))
	}

	// Rebuild the interval with the updated message
	updatedInterval := monitorapi.NewInterval(monitorapi.SourceNodeMonitor, monitorapi.Warning).
		Locator(intervalTmpl.BuildCondition().Locator).
		Message(msgBuilder).
		Display()

	return updatedInterval.Build(start, end)
}

func (*highCPUMetricCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *highCPUMetricCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	logger := logrus.WithField("MonitorTest", "HighCPUMetricCollector")

	// Filter for high CPU intervals
	highCPUIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceNodeMonitor &&
			eventInterval.Message.Reason == monitorapi.IntervalReason("HighCPUUsage")
	})

	logger.Infof("collected %d high CPU intervals for analysis", len(highCPUIntervals))

	// This monitor test is purely for data collection, not for generating test cases
	return nil, nil
}

func (*highCPUMetricCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*highCPUMetricCollector) Cleanup(ctx context.Context) error {
	return nil
}
