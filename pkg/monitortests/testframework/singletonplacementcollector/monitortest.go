// Package singletonplacementcollector records where single-replica openshift-* Deployment pods
// landed across the control plane nodes at the end of a run. Rolling master reboots during
// install and upgrade can leave most of these singletons on one master, and because their memory
// requests are far below actual usage the scheduler cannot see the imbalance (OCPBUGS-131892).
// This is data collection only; results are written to autodl for analysis in BigQuery.
package singletonplacementcollector

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/library-go/test/library/metrics"
	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/prometheus"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	prometheustypes "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

const (
	testName = "[Monitor:singleton-placement-collector][Jira:\"Test Framework\"] monitor test singleton-placement-collector collection"

	bytesPerMB = 1024 * 1024
)

type singletonPod struct {
	namespace          string
	deployment         string
	podName            string
	nodeName           string
	memoryRequestBytes int64
	podAge             time.Duration

	// nil when the metric could not be obtained for this pod
	currentUsageBytes *float64
	peakUsageBytes    *float64
	avgUsageBytes     *float64
}

type masterPlacement struct {
	nodeName           string
	singletonPods      int
	memoryRequestBytes int64

	// nil when the metric source was unavailable
	currentUsageBytes *float64
	peakUsageBytes    *float64
	avgUsageBytes     *float64
}

type usageStats struct {
	peakBytes float64
	avgBytes  float64
}

type singletonPlacementCollector struct {
	adminRESTConfig *rest.Config

	// Usage lookups are swappable so unit tests don't need a metrics API or Prometheus.
	// Both return values keyed by "namespace/pod".
	fetchCurrentUsage func(ctx context.Context, kubeClient kubernetes.Interface) (map[string]float64, error)
	fetchUsageStats   func(ctx context.Context, restConfig *rest.Config, kubeClient kubernetes.Interface, beginning, end time.Time) (map[string]usageStats, error)

	pods      []singletonPod
	placement []masterPlacement
}

func NewSingletonPlacementCollector() monitortestframework.MonitorTest {
	return &singletonPlacementCollector{
		fetchCurrentUsage: fetchCurrentUsageFromMetricsAPI,
		fetchUsageStats:   fetchUsageStatsFromPrometheus,
	}
}

func (w *singletonPlacementCollector) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *singletonPlacementCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *singletonPlacementCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	logger := logrus.WithField("MonitorTest", "SingletonPlacementCollector")

	if err := w.collect(ctx, beginning, end, time.Now()); err != nil {
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

	logger.Infof("collected %d singleton pods across %d masters", len(w.pods), len(w.placement))
	return nil, nil, nil
}

func (w *singletonPlacementCollector) collect(ctx context.Context, beginning, end, now time.Time) error {
	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return err
	}
	return w.collectFromClient(ctx, kubeClient, beginning, end, now)
}

func (w *singletonPlacementCollector) collectFromClient(ctx context.Context, kubeClient kubernetes.Interface, beginning, end, now time.Time) error {
	logger := logrus.WithField("MonitorTest", "SingletonPlacementCollector")

	masters, pods, err := listSingletonPods(ctx, kubeClient, now)
	if err != nil {
		return err
	}
	// HyperShift hosted clusters have no masters, and with one master there is no placement choice.
	if len(masters) < 2 {
		logger.Infof("found %d master nodes, skipping singleton placement collection", len(masters))
		return nil
	}

	// Usage metrics are best effort: placement and requests are still worth recording without them,
	// and a missing value shows up as NULL in BigQuery.
	currentUsage, err := w.fetchCurrentUsage(ctx, kubeClient)
	if err != nil {
		logger.WithError(err).Warn("unable to get current pod memory usage from metrics API")
		currentUsage = nil
	}
	stats, err := w.fetchUsageStats(ctx, w.adminRESTConfig, kubeClient, beginning, end)
	if err != nil {
		logger.WithError(err).Warn("unable to get peak/average pod memory usage from Prometheus")
		stats = nil
	}

	for i := range pods {
		key := pods[i].namespace + "/" + pods[i].podName
		if v, ok := currentUsage[key]; ok {
			pods[i].currentUsageBytes = &v
		}
		if s, ok := stats[key]; ok {
			pods[i].peakUsageBytes = &s.peakBytes
			pods[i].avgUsageBytes = &s.avgBytes
		}
	}

	w.pods = pods
	w.placement = summarizePlacement(masters, pods, len(currentUsage) > 0, len(stats) > 0)
	return nil
}

// listSingletonPods returns the sorted master node names and the running pods on them that belong
// to single-replica Deployments in openshift-* namespaces.
func listSingletonPods(ctx context.Context, kubeClient kubernetes.Interface, now time.Time) ([]string, []singletonPod, error) {
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	masterSet := map[string]bool{}
	for _, node := range nodes.Items {
		if isMaster(&node) {
			masterSet[node.Name] = true
		}
	}
	masters := make([]string, 0, len(masterSet))
	for name := range masterSet {
		masters = append(masters, name)
	}
	sort.Strings(masters)
	if len(masters) < 2 {
		return masters, nil, nil
	}

	deployments, err := kubeClient.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	singletonDeployments := map[string]bool{}
	for _, d := range deployments.Items {
		if !strings.HasPrefix(d.Namespace, "openshift-") {
			continue
		}
		// replicas defaults to 1 when unset
		if d.Spec.Replicas == nil || *d.Spec.Replicas == 1 {
			singletonDeployments[d.Namespace+"/"+d.Name] = true
		}
	}

	replicaSets, err := kubeClient.AppsV1().ReplicaSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list replicasets: %w", err)
	}
	rsToDeployment := map[string]string{}
	for _, rs := range replicaSets.Items {
		if owner := metav1.GetControllerOf(&rs); owner != nil && owner.Kind == "Deployment" {
			rsToDeployment[rs.Namespace+"/"+rs.Name] = owner.Name
		}
	}

	podList, err := kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list pods: %w", err)
	}
	var pods []singletonPod
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning || !masterSet[pod.Spec.NodeName] {
			continue
		}
		owner := metav1.GetControllerOf(&pod)
		if owner == nil || owner.Kind != "ReplicaSet" {
			continue
		}
		deployment, ok := rsToDeployment[pod.Namespace+"/"+owner.Name]
		if !ok || !singletonDeployments[pod.Namespace+"/"+deployment] {
			continue
		}
		var memoryRequest int64
		for _, c := range pod.Spec.Containers {
			if q, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
				memoryRequest += q.Value()
			}
		}
		pods = append(pods, singletonPod{
			namespace:          pod.Namespace,
			deployment:         deployment,
			podName:            pod.Name,
			nodeName:           pod.Spec.NodeName,
			memoryRequestBytes: memoryRequest,
			podAge:             now.Sub(pod.CreationTimestamp.Time),
		})
	}
	return masters, pods, nil
}

// summarizePlacement produces one entry per master, including masters with no singletons, so skew
// can be computed in BigQuery without knowing the master count. A master's usage sum is only set
// when at least one of its pods has a sample, so missing data loads as NULL rather than zero.
// Masters without singletons get a zero sum as long as the metric source returned data at all.
func summarizePlacement(masters []string, pods []singletonPod, haveCurrentUsage, haveUsageStats bool) []masterPlacement {
	byNode := map[string]*masterPlacement{}
	placement := make([]masterPlacement, len(masters))
	for i, name := range masters {
		placement[i].nodeName = name
		byNode[name] = &placement[i]
	}
	for _, pod := range pods {
		p := byNode[pod.nodeName]
		p.singletonPods++
		p.memoryRequestBytes += pod.memoryRequestBytes
		addSample(&p.currentUsageBytes, pod.currentUsageBytes)
		addSample(&p.peakUsageBytes, pod.peakUsageBytes)
		addSample(&p.avgUsageBytes, pod.avgUsageBytes)
	}
	for i := range placement {
		if placement[i].singletonPods > 0 {
			continue
		}
		if haveCurrentUsage {
			placement[i].currentUsageBytes = new(float64)
		}
		if haveUsageStats {
			placement[i].peakUsageBytes = new(float64)
			placement[i].avgUsageBytes = new(float64)
		}
	}
	return placement
}

func addSample(sum **float64, v *float64) {
	if v == nil {
		return
	}
	if *sum == nil {
		*sum = new(float64)
	}
	**sum += *v
}

func isMaster(node *corev1.Node) bool {
	if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
		return true
	}
	_, ok := node.Labels["node-role.kubernetes.io/control-plane"]
	return ok
}

// fetchCurrentUsageFromMetricsAPI returns the current working set bytes of every pod via the
// metrics.k8s.io resource metrics API, summed across containers.
func fetchCurrentUsageFromMetricsAPI(ctx context.Context, kubeClient kubernetes.Interface) (map[string]float64, error) {
	raw, err := kubeClient.CoreV1().RESTClient().Get().AbsPath("/apis/metrics.k8s.io/v1beta1/pods").DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	var podMetrics metricsv1beta1.PodMetricsList
	if err := json.Unmarshal(raw, &podMetrics); err != nil {
		return nil, fmt.Errorf("failed to decode pod metrics: %w", err)
	}
	return currentUsageFromPodMetrics(&podMetrics), nil
}

func currentUsageFromPodMetrics(podMetrics *metricsv1beta1.PodMetricsList) map[string]float64 {
	ret := map[string]float64{}
	for _, pm := range podMetrics.Items {
		var total int64
		for _, c := range pm.Containers {
			if q, ok := c.Usage[corev1.ResourceMemory]; ok {
				total += q.Value()
			}
		}
		ret[pm.Namespace+"/"+pm.Name] = float64(total)
	}
	return ret
}

// usageStatsQuery returns a single query covering every openshift-* pod, so cost does not grow
// with the number of singletons. Peak and average are distinguished by a "stat" label because
// "or" would otherwise drop the second series for each pod. Summing containers before taking
// max/avg gives the pod's true peak rather than a sum of per-container peaks.
func usageStatsQuery(window time.Duration) string {
	podWorkingSet := `sum by (namespace, pod) (container_memory_working_set_bytes{job="kubelet", metrics_path="/metrics/cadvisor", namespace=~"openshift-.*", container!="", image!=""})`
	rangeSelector := fmt.Sprintf("[%ds:1m]", int64(window.Seconds()))
	return fmt.Sprintf(`label_replace(max_over_time(%[1]s%[2]s), "stat", "peak", "", "") or label_replace(avg_over_time(%[1]s%[2]s), "stat", "avg", "", "")`,
		podWorkingSet, rangeSelector)
}

func fetchUsageStatsFromPrometheus(ctx context.Context, restConfig *rest.Config, kubeClient kubernetes.Interface, beginning, end time.Time) (map[string]usageStats, error) {
	window := end.Sub(beginning)
	if window < time.Minute {
		return nil, fmt.Errorf("monitoring window %s is too short", window)
	}

	if _, err := kubeClient.CoreV1().Namespaces().Get(ctx, "openshift-monitoring", metav1.GetOptions{}); apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("openshift-monitoring namespace not found")
	} else if err != nil {
		return nil, err
	}
	routeClient, err := routeclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	prometheusClient, err := metrics.NewPrometheusClient(ctx, kubeClient, routeClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus client: %w", err)
	}
	if _, err := prometheus.EnsureThanosQueriersConnectedToPromSidecars(ctx, prometheusClient); err != nil {
		return nil, err
	}

	result, warnings, err := prometheusClient.Query(ctx, usageStatsQuery(window), end)
	if err != nil {
		return nil, fmt.Errorf("failed to query pod memory usage: %w", err)
	}
	for _, warning := range warnings {
		logrus.WithField("MonitorTest", "SingletonPlacementCollector").Warnf("pod memory usage query warning: %s", warning)
	}
	return usageStatsFromQueryResult(result)
}

func usageStatsFromQueryResult(result prometheustypes.Value) (map[string]usageStats, error) {
	vector, ok := result.(prometheustypes.Vector)
	if !ok {
		return nil, fmt.Errorf("expected vector result, got %s", result.Type())
	}
	ret := map[string]usageStats{}
	for _, sample := range vector {
		key := string(sample.Metric["namespace"]) + "/" + string(sample.Metric["pod"])
		s := ret[key]
		switch sample.Metric["stat"] {
		case "peak":
			s.peakBytes = float64(sample.Value)
		case "avg":
			s.avgBytes = float64(sample.Value)
		}
		ret[key] = s
	}
	return ret, nil
}

func (*singletonPlacementCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*singletonPlacementCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	// This monitor test is purely for data collection, not for generating test cases
	return nil, nil
}

func (w *singletonPlacementCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	logger := logrus.WithField("func", "WriteContentToStorage")

	if len(w.placement) == 0 {
		logger.Info("No singleton placement data to export")
		return nil
	}

	podRows := make([]map[string]string, 0, len(w.pods))
	for _, pod := range w.pods {
		row := map[string]string{
			"Namespace":       pod.namespace,
			"Deployment":      pod.deployment,
			"PodName":         pod.podName,
			"NodeName":        pod.nodeName,
			"MemoryRequestMB": formatMB(float64(pod.memoryRequestBytes)),
			"PodAgeSeconds":   fmt.Sprintf("%d", int64(pod.podAge.Seconds())),
		}
		setMB(row, "MemoryUsageMB", pod.currentUsageBytes)
		setMB(row, "PeakMemoryUsageMB", pod.peakUsageBytes)
		setMB(row, "AvgMemoryUsageMB", pod.avgUsageBytes)
		podRows = append(podRows, row)
	}
	podsFile := dataloader.DataFile{
		TableName: "control_plane_singleton_pods",
		Schema: map[string]dataloader.DataType{
			"Namespace":         dataloader.DataTypeString,
			"Deployment":        dataloader.DataTypeString,
			"PodName":           dataloader.DataTypeString,
			"NodeName":          dataloader.DataTypeString,
			"MemoryRequestMB":   dataloader.DataTypeFloat64,
			"MemoryUsageMB":     dataloader.DataTypeFloat64,
			"PeakMemoryUsageMB": dataloader.DataTypeFloat64,
			"AvgMemoryUsageMB":  dataloader.DataTypeFloat64,
			"PodAgeSeconds":     dataloader.DataTypeInteger,
		},
		Rows: podRows,
	}

	placementRows := make([]map[string]string, 0, len(w.placement))
	for _, p := range w.placement {
		row := map[string]string{
			"NodeName":                 p.nodeName,
			"SingletonPods":            fmt.Sprintf("%d", p.singletonPods),
			"SingletonMemoryRequestMB": formatMB(float64(p.memoryRequestBytes)),
			"TotalMasters":             fmt.Sprintf("%d", len(w.placement)),
		}
		setMB(row, "SingletonMemoryUsageMB", p.currentUsageBytes)
		setMB(row, "SingletonPeakMemoryUsageMB", p.peakUsageBytes)
		setMB(row, "SingletonAvgMemoryUsageMB", p.avgUsageBytes)
		placementRows = append(placementRows, row)
	}
	placementFile := dataloader.DataFile{
		TableName: "control_plane_singleton_placement",
		Schema: map[string]dataloader.DataType{
			"NodeName":                   dataloader.DataTypeString,
			"SingletonPods":              dataloader.DataTypeInteger,
			"SingletonMemoryRequestMB":   dataloader.DataTypeFloat64,
			"SingletonMemoryUsageMB":     dataloader.DataTypeFloat64,
			"SingletonPeakMemoryUsageMB": dataloader.DataTypeFloat64,
			"SingletonAvgMemoryUsageMB":  dataloader.DataTypeFloat64,
			"TotalMasters":               dataloader.DataTypeInteger,
		},
		Rows: placementRows,
	}

	for _, f := range []struct {
		prefix string
		data   dataloader.DataFile
	}{
		{prefix: "control-plane-singleton-pods", data: podsFile},
		{prefix: "control-plane-singleton-placement", data: placementFile},
	} {
		fileName := filepath.Join(storageDir, fmt.Sprintf("%s%s-%s", f.prefix, timeSuffix, dataloader.AutoDataLoaderSuffix))
		if err := dataloader.WriteDataFile(fileName, f.data); err != nil {
			logger.WithError(err).Warnf("Failed to write autodl file: %s", fileName)
			return err
		}
		logger.Infof("Wrote %d rows to autodl file: %s", len(f.data.Rows), fileName)
	}
	return nil
}

func (*singletonPlacementCollector) Cleanup(ctx context.Context) error {
	return nil
}

func formatMB(bytes float64) string {
	return fmt.Sprintf("%.2f", bytes/bytesPerMB)
}

// setMB leaves the column out of the row when the value is unknown so it loads as NULL.
func setMB(row map[string]string, column string, bytes *float64) {
	if bytes != nil {
		row[column] = formatMB(*bytes)
	}
}
