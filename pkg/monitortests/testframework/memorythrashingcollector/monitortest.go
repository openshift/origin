package memorythrashingcollector

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
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

// memoryThrashingCollector records intervals where a node is thrashing memory: a high major
// page fault rate together with a high rate of data paged back in from disk.
//
// A major page fault means a process touched a page that was no longer in memory and had to
// be read back from disk. Nodes have no swap, so under memory pressure the kernel evicts file
// pages (binaries, libraries, mmapped files such as etcd's database) and processes immediately
// fault them back in. Healthy nodes stay in the low single digits per second; thrashing nodes
// sustain hundreds to thousands.
//
// The fault rate alone doesn't say how hard the disk is hit. Bursts of faults on small pages
// read only a few MB/s and have shown no harm in CI, while the cases that stalled etcd and the
// apiserver read back 100+ MB/s and saturated the disk. Both thresholds must be exceeded.
type memoryThrashingCollector struct {
	adminRESTConfig *rest.Config
	// majorFaultsThreshold is the major page fault rate per second above which a node may be
	// thrashing.
	majorFaultsThreshold float64
	// pageInThreshold is the rate of data paged in from disk, in MB/s, above which the faults
	// are heavy enough to saturate the disk.
	pageInThreshold float64
	intervals       []thrashingInterval
}

// queryStep is the sample spacing of the range queries. Each sample above the thresholds counts
// for one full step, so an interval ends one step after its last such sample and a single
// sample still has a 30s duration.
const queryStep = 30 * time.Second

type thrashingInterval struct {
	nodeName   string
	nodeRole   string
	start      time.Time
	end        time.Time
	peakFaults float64
	peakPageIn float64 // MB/s
}

// NewMemoryThrashingCollector returns a monitor test that records memory thrashing intervals per node.
func NewMemoryThrashingCollector() monitortestframework.MonitorTest {
	return &memoryThrashingCollector{
		majorFaultsThreshold: 50.0,
		pageInThreshold:      50.0,
	}
}

func (w *memoryThrashingCollector) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *memoryThrashingCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *memoryThrashingCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	const testName = "[Monitor:memory-thrashing-collector][Jira:\"Node / Kubelet\"] monitor test memory-thrashing-collector collection"
	logger := logrus.WithField("MonitorTest", "MemoryThrashingCollector")

	intervals, err := w.collectFromPrometheus(ctx, w.adminRESTConfig, beginning)
	if err != nil {
		// Thanos queriers may temporarily lose connectivity to Prometheus sidecars after a
		// disruptive operation (e.g. CA rotation) that restarts TLS connections. Treat this
		// as a flake rather than a hard failure so disruptive test suites are not blocked.
		logger.WithError(err).Warn("failed during collection; recording as flake")
		return nil, []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("failed during collection\n%v", err),
				},
			},
			// second entry with same name = flake pattern
			{Name: testName},
		}, nil
	}

	logger.Infof("collected %d memory thrashing intervals", len(intervals))
	return intervals, nil, nil
}

// collectFromPrometheus connects to the in-cluster Prometheus and builds the thrashing intervals
// since startTime. Clusters without openshift-monitoring produce no intervals.
func (w *memoryThrashingCollector) collectFromPrometheus(ctx context.Context, restConfig *rest.Config, startTime time.Time) ([]monitorapi.Interval, error) {
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
		return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
	}

	if intervals, err := prometheus.EnsureThanosQueriersConnectedToPromSidecars(ctx, prometheusClient); err != nil {
		return intervals, fmt.Errorf("failed to check Thanos querier connection to Prometheus sidecars: %w", err)
	}

	nodeList, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.WithError(err).Warn("Failed to list nodes, node role information may be incomplete")
		nodeList = &corev1.NodeList{}
	}

	return w.collectFromPrometheusClient(ctx, prometheusClient, buildNodeInfoMap(nodeList), startTime, time.Now())
}

// collectFromPrometheusClient queries the major fault and page-in rates for every node and
// builds the thrashing intervals between startTime and endTime.
func (w *memoryThrashingCollector) collectFromPrometheusClient(ctx context.Context, prometheusClient prometheusv1.API, nodeInfoMap map[string]nodeInfo, startTime, endTime time.Time) ([]monitorapi.Interval, error) {
	logger := logrus.WithField("func", "collectFromPrometheusClient")

	timeRange := prometheusv1.Range{
		Start: startTime,
		End:   endTime,
		Step:  queryStep,
	}

	// node-exporter's instance label is the node name. pgpgin is in KB.
	faults, err := queryRange(ctx, prometheusClient, logger, `rate(node_vmstat_pgmajfault[1m])`, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to query major page fault rate: %w", err)
	}
	pageIn, err := queryRange(ctx, prometheusClient, logger, `rate(node_vmstat_pgpgin[1m]) / 1024`, timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to query page-in rate: %w", err)
	}

	return w.createIntervalsFromMetrics(logger, faults, pageIn, nodeInfoMap), nil
}

// queryRange runs a range query and logs any warnings Prometheus returns.
func queryRange(ctx context.Context, prometheusClient prometheusv1.API, logger logrus.FieldLogger, query string, timeRange prometheusv1.Range) (prometheustypes.Value, error) {
	result, warnings, err := prometheusClient.QueryRange(ctx, query, timeRange)
	if err != nil {
		return nil, err
	}
	for _, warning := range warnings {
		logger.Warnf("query %q warning: %s", query, warning)
	}
	return result, nil
}

// createIntervalsFromMetrics builds an interval for each run of consecutive samples where a
// node's major fault rate and page-in rate are both above their thresholds.
func (w *memoryThrashingCollector) createIntervalsFromMetrics(logger logrus.FieldLogger, faults, pageIn prometheustypes.Value, nodeInfoMap map[string]nodeInfo) []monitorapi.Interval {
	ret := []monitorapi.Interval{}

	faultsMatrix, ok := faults.(prometheustypes.Matrix)
	if !ok {
		logger.WithField("type", faults.Type()).Warning("unhandled prometheus value type received")
		return ret
	}

	// page-in rate (MB/s) by instance and sample time
	pageInByInstance := map[string]map[prometheustypes.Time]float64{}
	if pageInMatrix, ok := pageIn.(prometheustypes.Matrix); ok {
		for _, stream := range pageInMatrix {
			samples := map[prometheustypes.Time]float64{}
			for _, sample := range stream.Values {
				samples[sample.Timestamp] = float64(sample.Value)
			}
			pageInByInstance[string(stream.Metric["instance"])] = samples
		}
	}

	for _, promSampleStream := range faultsMatrix {
		instance := string(promSampleStream.Metric["instance"])
		info := nodeInfoMap[instance]
		if info.name == "" {
			// Fallback to instance if name not found (transient nodes may not be mapped)
			info.name = instance
		}

		// Track consecutive samples above the thresholds
		var current *thrashingInterval
		for _, sample := range promSampleStream.Values {
			ts := sample.Timestamp.Time()
			faultRate := float64(sample.Value)
			pageInRate := pageInByInstance[instance][sample.Timestamp]
			// Prometheus omits steps with no value (e.g. failed scrapes), so a gap ends the
			// current interval rather than bridging it.
			if current != nil && ts.After(current.end) {
				ret = append(ret, w.recordInterval(*current))
				current = nil
			}
			if faultRate > w.majorFaultsThreshold && pageInRate > w.pageInThreshold {
				if current == nil {
					current = &thrashingInterval{nodeName: info.name, nodeRole: info.nodeRole, start: ts}
				}
				current.end = ts.Add(queryStep)
				current.peakFaults = max(current.peakFaults, faultRate)
				current.peakPageIn = max(current.peakPageIn, pageInRate)
				continue
			}
			if current != nil {
				ret = append(ret, w.recordInterval(*current))
				current = nil
			}
		}
		// Handle a thrashing period that extends to the end of the monitoring window
		if current != nil {
			ret = append(ret, w.recordInterval(*current))
		}
	}

	return ret
}

// recordInterval keeps the interval for the autodl file and builds the monitor interval for it.
func (w *memoryThrashingCollector) recordInterval(t thrashingInterval) monitorapi.Interval {
	w.intervals = append(w.intervals, t)

	msg := monitorapi.NewMessage().
		Reason(monitorapi.IntervalReason("MemoryThrashing")).
		HumanMessage(fmt.Sprintf("major page faults above %.0f/s with over %.0f MB/s paged in from disk on node %s, indicating memory thrashing", w.majorFaultsThreshold, w.pageInThreshold, t.nodeName)).
		WithAnnotation("major_faults_threshold", fmt.Sprintf("%.0f", w.majorFaultsThreshold)).
		WithAnnotation("page_in_threshold_mb_per_second", fmt.Sprintf("%.0f", w.pageInThreshold)).
		WithAnnotation("peak_major_faults_per_second", fmt.Sprintf("%.1f", t.peakFaults)).
		WithAnnotation("peak_page_in_mb_per_second", fmt.Sprintf("%.1f", t.peakPageIn))

	return monitorapi.NewInterval(monitorapi.SourceMemoryMonitor, monitorapi.Warning).
		Locator(monitorapi.NewLocator().NodeFromNameWithRole(t.nodeName, t.nodeRole)).
		Message(msg).
		Display().
		Build(t.start, t.end)
}

func (*memoryThrashingCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

const memoryThrashingTestName = "[sig-node] nodes should not be thrashing memory"

// EvaluateTestsFromConstructedIntervals flakes (a failure plus a pass) when any node thrashed
// memory, so we can track which jobs hit it and how often without failing them.
func (w *memoryThrashingCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	thrashing := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceMemoryMonitor
	})

	junits := []*junitapi.JUnitTestCase{}
	if len(thrashing) > 0 {
		lines := []string{}
		for _, interval := range thrashing {
			lines = append(lines, interval.String())
		}
		junits = append(junits, &junitapi.JUnitTestCase{
			Name: memoryThrashingTestName,
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("found nodes thrashing memory:\n  %s", strings.Join(lines, "\n  ")),
			},
		})
	}
	// Add a success so this is marked as a flake at worst
	return append(junits, &junitapi.JUnitTestCase{Name: memoryThrashingTestName}), nil
}

// WriteContentToStorage writes the thrashing intervals to the node_memory_thrashing_intervals autodl file.
func (w *memoryThrashingCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	logger := logrus.WithField("func", "WriteContentToStorage")

	// Runs without thrashing write no rows. Per-run totals, including zero, are recorded by
	// the interval-duration-sum monitor test under the MemoryMonitor source.
	if len(w.intervals) == 0 {
		logger.Info("No memory thrashing intervals to export")
		return nil
	}

	rows := make([]map[string]string, 0, len(w.intervals))
	for _, t := range w.intervals {
		rows = append(rows, map[string]string{
			"NodeName":                 t.nodeName,
			"NodeRole":                 t.nodeRole,
			"StartTime":                t.start.Format(time.RFC3339),
			"EndTime":                  t.end.Format(time.RFC3339),
			"DurationSeconds":          fmt.Sprintf("%d", int(t.end.Sub(t.start).Seconds())),
			"PeakMajorFaultsPerSecond": fmt.Sprintf("%.1f", t.peakFaults),
			"PeakPageInMBPerSecond":    fmt.Sprintf("%.1f", t.peakPageIn),
		})
	}

	dataFile := dataloader.DataFile{
		TableName: "node_memory_thrashing_intervals",
		Schema: map[string]dataloader.DataType{
			"NodeName":                 dataloader.DataTypeString,
			"NodeRole":                 dataloader.DataTypeString,
			"StartTime":                dataloader.DataTypeTimestamp,
			"EndTime":                  dataloader.DataTypeTimestamp,
			"DurationSeconds":          dataloader.DataTypeInteger,
			"PeakMajorFaultsPerSecond": dataloader.DataTypeFloat64,
			"PeakPageInMBPerSecond":    dataloader.DataTypeFloat64,
		},
		Rows: rows,
	}

	fileName := filepath.Join(storageDir, fmt.Sprintf("node-memory-thrashing-intervals%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))
	if err := dataloader.WriteDataFile(fileName, dataFile); err != nil {
		logger.WithError(err).Warnf("Failed to write memory thrashing autodl file: %s", fileName)
		return err
	}

	logger.Infof("Wrote %d memory thrashing intervals to autodl file: %s", len(rows), fileName)
	return nil
}

func (*memoryThrashingCollector) Cleanup(ctx context.Context) error {
	return nil
}

// nodeInfo contains information about a node
type nodeInfo struct {
	name     string
	nodeRole string
}

// buildNodeInfoMap creates a map from node name and internal IP addresses to node information
func buildNodeInfoMap(nodeList *corev1.NodeList) map[string]nodeInfo {
	nodeInfoMap := make(map[string]nodeInfo)

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		info := nodeInfo{
			name:     node.Name,
			nodeRole: getNodeRole(node),
		}

		nodeInfoMap[node.Name] = info
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				nodeInfoMap[address.Address] = info
				nodeInfoMap[address.Address+":9100"] = info
			}
		}
	}

	return nodeInfoMap
}

// getNodeRole determines if a node is a master or worker based on its labels
func getNodeRole(node *corev1.Node) string {
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
