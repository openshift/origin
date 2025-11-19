package cpumetriccollector

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/library-go/test/library/metrics"
	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/prometheus"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prometheustypes "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type cpuDataPoint struct {
	timestamp time.Time
	nodeName  string
	nodeType  string
	cpuUsage  float64
}

type cpuMetricCollector struct {
	adminRESTConfig  *rest.Config
	highCPUThreshold float64
	cpuDataPoints    []cpuDataPoint
}

func NewCPUMetricCollector() monitortestframework.MonitorTest {
	return &cpuMetricCollector{
		highCPUThreshold: 95.0, // Default to 95% threshold
	}
}

func (w *cpuMetricCollector) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *cpuMetricCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *cpuMetricCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	logger := logrus.WithField("MonitorTest", "CPUMetricCollector")

	intervals, err := w.collectCPUMetricsFromPrometheus(ctx, w.adminRESTConfig, beginning)
	if err != nil {
		return nil, nil, err
	}

	logger.Infof("collected %d high CPU intervals", len(intervals))
	return intervals, nil, nil
}

func (w *cpuMetricCollector) collectCPUMetricsFromPrometheus(ctx context.Context, restConfig *rest.Config, startTime time.Time) ([]monitorapi.Interval, error) {
	logger := logrus.WithField("func", "collectCPUMetricsFromPrometheus")
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

	// Get node information for determining node types
	nodeList, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.WithError(err).Warn("Failed to list nodes, node type information may be incomplete")
		nodeList = &corev1.NodeList{}
	}

	nodeInfoMap := buildNodeInfoMap(nodeList)

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

	// Collect CPU data points for timeline export
	w.collectCPUDataPointsFromMetrics(cpuMetrics, nodeInfoMap)

	// Create intervals for high CPU periods
	return w.createIntervalsFromCPUMetrics(logger, cpuMetrics)
}

func (w *cpuMetricCollector) collectCPUDataPointsFromMetrics(promVal prometheustypes.Value, nodeInfoMap map[string]nodeInfo) {
	switch {
	case promVal.Type() == prometheustypes.ValMatrix:
		promMatrix := promVal.(prometheustypes.Matrix)
		for _, promSampleStream := range promMatrix {
			instance := string(promSampleStream.Metric["instance"])

			// Get node information from the map
			nodeInfo := nodeInfoMap[instance]
			nodeName := nodeInfo.name
			if nodeName == "" {
				nodeName = instance // Fallback to instance if name not found
			}

			// Collect all CPU data points for timeline export
			for _, currValue := range promSampleStream.Values {
				currTime := currValue.Timestamp.Time()
				cpuUsage := float64(currValue.Value)

				w.cpuDataPoints = append(w.cpuDataPoints, cpuDataPoint{
					timestamp: currTime,
					nodeName:  nodeName,
					nodeType:  nodeInfo.nodeType,
					cpuUsage:  cpuUsage,
				})
			}
		}
	}
}

func (w *cpuMetricCollector) createIntervalsFromCPUMetrics(logger logrus.FieldLogger, promVal prometheustypes.Value) ([]monitorapi.Interval, error) {
	ret := []monitorapi.Interval{}

	switch {
	case promVal.Type() == prometheustypes.ValMatrix:
		promMatrix := promVal.(prometheustypes.Matrix)
		for _, promSampleStream := range promMatrix {
			instance := string(promSampleStream.Metric["instance"])

			// Create locator for the node
			locator := monitorapi.NewLocator().NodeFromName(instance)

			// Track consecutive high CPU periods
			var highCPUStart *time.Time
			var highCPUEnd *time.Time
			var peakCPUUsage float64

			for _, currValue := range promSampleStream.Values {
				currTime := currValue.Timestamp.Time()
				cpuUsage := float64(currValue.Value)

				// Check if CPU usage exceeds threshold
				if cpuUsage > w.highCPUThreshold {
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
						ret = append(ret, w.createCPUInterval(locator, instance, *highCPUStart, *highCPUEnd, peakCPUUsage))
						// Reset tracking variables
						highCPUStart = nil
						highCPUEnd = nil
						peakCPUUsage = 0
					}
				}
			}

			// Handle case where high CPU period extends to the end of the monitoring window
			if highCPUStart != nil && highCPUEnd != nil {
				ret = append(ret, w.createCPUInterval(locator, instance, *highCPUStart, *highCPUEnd, peakCPUUsage))
			}
		}

	default:
		logger.WithField("type", promVal.Type()).Warning("unhandled prometheus value type received")
	}

	return ret, nil
}

func (w *cpuMetricCollector) createCPUInterval(locator monitorapi.Locator, instance string, start, end time.Time, cpuUsage float64) monitorapi.Interval {
	// Create message with all necessary information
	msgBuilder := monitorapi.NewMessage().
		Reason(monitorapi.IntervalReason("HighCPUUsage")).
		HumanMessage(fmt.Sprintf("CPU usage above %.1f%% threshold on instance %s", w.highCPUThreshold, instance)).
		WithAnnotation("cpu_threshold", fmt.Sprintf("%.1f", w.highCPUThreshold))

	if cpuUsage > 0 {
		msgBuilder = msgBuilder.WithAnnotation("peak_cpu_usage", fmt.Sprintf("%.2f", cpuUsage))
	}

	// Create and build the interval directly
	interval := monitorapi.NewInterval(monitorapi.SourceCPUMonitor, monitorapi.Warning).
		Locator(locator).
		Message(msgBuilder).
		Display()

	return interval.Build(start, end)
}

func (*cpuMetricCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *cpuMetricCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	// This monitor test is purely for data collection, not for generating test cases
	return nil, nil
}

func (w *cpuMetricCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	logger := logrus.WithField("func", "WriteContentToStorage")

	if len(w.cpuDataPoints) == 0 {
		logger.Info("No CPU data points to export")
		return nil
	}

	// Convert CPU data points to autodl rows
	rows := make([]map[string]string, 0, len(w.cpuDataPoints))
	for _, dp := range w.cpuDataPoints {
		rows = append(rows, map[string]string{
			"Timestamp": dp.timestamp.Format(time.RFC3339),
			"NodeName":  dp.nodeName,
			"NodeType":  dp.nodeType,
			"CPUUsage":  fmt.Sprintf("%.2f", dp.cpuUsage),
		})
	}

	// Create autodl data file
	dataFile := dataloader.DataFile{
		TableName: "node_cpu_usage_timeline",
		Schema: map[string]dataloader.DataType{
			"Timestamp": dataloader.DataTypeTimestamp,
			"NodeName":  dataloader.DataTypeString,
			"NodeType":  dataloader.DataTypeString,
			"CPUUsage":  dataloader.DataTypeFloat64,
		},
		Rows: rows,
	}

	// Write the autodl file
	fileName := filepath.Join(storageDir, fmt.Sprintf("node-cpu-usage-timeline%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))
	err := dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logger.WithError(err).Warnf("Failed to write CPU timeline autodl file: %s", fileName)
		return err
	}

	logger.Infof("Wrote %d CPU data points to autodl file: %s", len(rows), fileName)
	return nil
}

func (*cpuMetricCollector) Cleanup(ctx context.Context) error {
	return nil
}

// nodeInfo contains information about a node
type nodeInfo struct {
	name     string
	nodeType string
}

// buildNodeInfoMap creates a map from node IP addresses to node information
func buildNodeInfoMap(nodeList *corev1.NodeList) map[string]nodeInfo {
	nodeInfoMap := make(map[string]nodeInfo)

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		nodeType := getNodeType(node)

		// Map all node IP addresses to the node info
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				// Prometheus typically uses IP:port format
				nodeInfoMap[address.Address] = nodeInfo{
					name:     node.Name,
					nodeType: nodeType,
				}
				// Also map with port suffix (common Prometheus format)
				nodeInfoMap[address.Address+":9100"] = nodeInfo{
					name:     node.Name,
					nodeType: nodeType,
				}
			}
		}
	}

	return nodeInfoMap
}

// getNodeType determines if a node is a master or worker based on its labels
func getNodeType(node *corev1.Node) string {
	if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
		return "master"
	}
	if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
		return "master"
	}
	if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok {
		return "worker"
	}
	return "unknown"
}
