// Package singletonplacementcollector records every running openshift-* pod on the control plane
// nodes at the end of a run (Deployments of any replica count, DaemonSets, static pods, etc.),
// along with their memory usage before the e2e suite started and during it. Rolling master reboots
// during install and upgrade can leave most single-replica Deployments on one master, and because
// memory requests are far below actual usage the scheduler cannot see the imbalance
// (OCPBUGS-131892). Covering all pods shows each master's full memory picture, not just the part
// contributed by singletons.
//
// Despite the "singleton" naming, which is kept because it is baked into the autodl table and
// column names, the collector covers all control plane pods. The placement table's Singleton*
// columns and RebalanceSingletons remain limited to single-replica Deployments.
// This is data collection only; results are written to autodl for analysis in BigQuery.
package singletonplacementcollector

import (
	"context"
	"encoding/json"
	"errors"
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
	"k8s.io/utils/ptr"
)

const (
	testName = "[Monitor:singleton-placement-collector][Jira:\"Test Framework\"] monitor test singleton-placement-collector collection"

	bytesPerMB = 1024 * 1024
)

// Workload kinds for pods that are not owned by a regular controller.
const (
	workloadKindStaticPod = "StaticPod"
	workloadKindPod       = "Pod"
)

type controlPlanePod struct {
	namespace string
	// workloadKind is the controlling workload: Deployment (resolved through its ReplicaSet),
	// DaemonSet, StaticPod for mirror pods, Pod for pods without a controller (e.g. guard pods),
	// or the controller's kind otherwise.
	workloadKind string
	// workload names the controller. Static and controller-less pods are named after their node, so
	// the node suffix is removed to get a name shared by the same workload on every master.
	workload string
	// replicas is the Deployment's desired replica count; nil for other workload kinds.
	replicas           *int32
	podName            string
	nodeName           string
	memoryRequestBytes int64
	podAge             time.Duration

	// nil when the metric could not be obtained for this pod
	startUsageBytes   *float64
	currentUsageBytes *float64
	peakUsageBytes    *float64
	avgUsageBytes     *float64
}

type masterPlacement struct {
	nodeName string
	// all covers every control plane pod on the node; singletons only those from single-replica
	// Deployments, matching what the collector recorded before it covered all pods.
	all        podTotals
	singletons podTotals
}

type podTotals struct {
	pods               int
	memoryRequestBytes int64

	// nil when the metric source was unavailable
	startUsageBytes   *float64
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

	// startPodUsage is the memory usage of each pod before the e2e suite ran, keyed by
	// "namespace/pod". startWorkloadUsage is the mean per-pod usage of each workload, keyed by
	// workloadKey, and is used for pods replaced during the run (most of them, on upgrades).
	// For singletons this is exactly the previous pod's usage.
	startPodUsage      map[string]float64
	startWorkloadUsage map[string]float64
	startErr           error

	pods      []controlPlanePod
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
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		w.startErr = err
		return nil
	}
	// Kubernetes API failures are reported from CollectData so they surface as a flake rather than
	// failing the run.
	w.startErr = w.collectStartUsage(ctx, kubeClient, time.Now())
	return nil
}

// collectStartUsage records the steady state memory usage of the control plane pods after install,
// for tuning memory requests.
func (w *singletonPlacementCollector) collectStartUsage(ctx context.Context, kubeClient kubernetes.Interface, now time.Time) error {
	masters, pods, err := listControlPlanePods(ctx, kubeClient, now)
	if err != nil {
		return fmt.Errorf("failed to list control plane pods at start: %w", err)
	}
	if len(masters) == 0 {
		return nil
	}
	// Best effort like the end of run usage: a metrics API outage leaves the start columns NULL.
	usage, err := w.fetchCurrentUsage(ctx, kubeClient)
	if err != nil {
		logrus.WithField("MonitorTest", "SingletonPlacementCollector").WithError(err).Warn("unable to get pod memory usage at start from metrics API")
		return nil
	}
	w.startPodUsage = map[string]float64{}
	sums := map[string]float64{}
	counts := map[string]int{}
	for _, pod := range pods {
		v, ok := usage[pod.namespace+"/"+pod.podName]
		if !ok {
			continue
		}
		w.startPodUsage[pod.namespace+"/"+pod.podName] = v
		sums[pod.workloadKey()] += v
		counts[pod.workloadKey()]++
	}
	w.startWorkloadUsage = map[string]float64{}
	for key, sum := range sums {
		w.startWorkloadUsage[key] = sum / float64(counts[key])
	}
	return nil
}

func (p *controlPlanePod) isSingleton() bool {
	return p.workloadKind == "Deployment" && p.replicas != nil && *p.replicas == 1
}

func (p *controlPlanePod) workloadKey() string {
	return p.namespace + "/" + p.workloadKind + "/" + p.workload
}

func (w *singletonPlacementCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	logger := logrus.WithField("MonitorTest", "SingletonPlacementCollector")

	if err := errors.Join(w.startErr, w.collect(ctx, beginning, end, time.Now())); err != nil {
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

	logger.Infof("collected %d control plane pods across %d masters", len(w.pods), len(w.placement))
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

	masters, pods, err := listControlPlanePods(ctx, kubeClient, now)
	if err != nil {
		return err
	}
	// HyperShift hosted clusters have no masters. Single node and two node topologies have little or
	// no placement choice, but their per-pod memory usage is still worth recording.
	if len(masters) == 0 {
		logger.Infof("found %d master nodes, skipping control plane pod collection", len(masters))
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
		if v, ok := w.startPodUsage[key]; ok {
			pods[i].startUsageBytes = &v
		} else if v, ok := w.startWorkloadUsage[pods[i].workloadKey()]; ok {
			pods[i].startUsageBytes = &v
		}
		if v, ok := currentUsage[key]; ok {
			pods[i].currentUsageBytes = &v
		}
		if s, ok := stats[key]; ok {
			pods[i].peakUsageBytes = &s.peakBytes
			pods[i].avgUsageBytes = &s.avgBytes
		}
	}

	w.pods = pods
	w.placement = summarizePlacement(masters, pods, len(w.startPodUsage) > 0, len(currentUsage) > 0, len(stats) > 0)
	return nil
}

// listSingletonPods returns the sorted master node names and the pods on them that belong to
// single-replica Deployments, the only pods RebalanceSingletons may move.
func listSingletonPods(ctx context.Context, kubeClient kubernetes.Interface, now time.Time) ([]string, []controlPlanePod, error) {
	masters, pods, err := listControlPlanePods(ctx, kubeClient, now)
	if err != nil {
		return nil, nil, err
	}
	var singletons []controlPlanePod
	for _, pod := range pods {
		if pod.isSingleton() {
			singletons = append(singletons, pod)
		}
	}
	return masters, singletons, nil
}

// listControlPlanePods returns the sorted master node names and every running pod on them in an
// openshift-* namespace. Other namespaces are skipped to keep out transient e2e test pods.
func listControlPlanePods(ctx context.Context, kubeClient kubernetes.Interface, now time.Time) ([]string, []controlPlanePod, error) {
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
	if len(masters) == 0 {
		return masters, nil, nil
	}

	deployments, err := kubeClient.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	deploymentReplicas := map[string]int32{}
	for _, d := range deployments.Items {
		// replicas defaults to 1 when unset
		deploymentReplicas[d.Namespace+"/"+d.Name] = ptr.Deref(d.Spec.Replicas, 1)
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
	var pods []controlPlanePod
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning || !masterSet[pod.Spec.NodeName] || !strings.HasPrefix(pod.Namespace, "openshift-") {
			continue
		}
		cpPod := controlPlanePod{
			namespace: pod.Namespace,
			podName:   pod.Name,
			nodeName:  pod.Spec.NodeName,
			podAge:    now.Sub(pod.CreationTimestamp.Time),
		}
		switch owner := metav1.GetControllerOf(&pod); {
		case owner == nil:
			cpPod.workloadKind = workloadKindPod
			cpPod.workload = strings.TrimSuffix(pod.Name, "-"+pod.Spec.NodeName)
		case owner.Kind == "Node":
			cpPod.workloadKind = workloadKindStaticPod
			cpPod.workload = strings.TrimSuffix(pod.Name, "-"+pod.Spec.NodeName)
		case owner.Kind == "ReplicaSet":
			if deployment, ok := rsToDeployment[pod.Namespace+"/"+owner.Name]; ok {
				cpPod.workloadKind = "Deployment"
				cpPod.workload = deployment
				if replicas, ok := deploymentReplicas[pod.Namespace+"/"+deployment]; ok {
					cpPod.replicas = &replicas
				}
			} else {
				cpPod.workloadKind = owner.Kind
				cpPod.workload = owner.Name
			}
		default:
			cpPod.workloadKind = owner.Kind
			cpPod.workload = owner.Name
		}
		for _, c := range pod.Spec.Containers {
			if q, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
				cpPod.memoryRequestBytes += q.Value()
			}
		}
		pods = append(pods, cpPod)
	}
	return masters, pods, nil
}

// summarizePlacement produces one entry per master, including masters with no pods, so skew can be
// computed in BigQuery without knowing the master count. A master's usage sum is only set when at
// least one of its counted pods has a sample, so missing data loads as NULL rather than zero.
// Totals with no pods get a zero sum as long as the metric source returned data at all.
// The start usage sum covers the pods on the master at the end, not those there at start.
func summarizePlacement(masters []string, pods []controlPlanePod, haveStartUsage, haveCurrentUsage, haveUsageStats bool) []masterPlacement {
	byNode := map[string]*masterPlacement{}
	placement := make([]masterPlacement, len(masters))
	for i, name := range masters {
		placement[i].nodeName = name
		byNode[name] = &placement[i]
	}
	for _, pod := range pods {
		p := byNode[pod.nodeName]
		p.all.add(pod)
		if pod.isSingleton() {
			p.singletons.add(pod)
		}
	}
	for i := range placement {
		for _, t := range []*podTotals{&placement[i].all, &placement[i].singletons} {
			if t.pods > 0 {
				continue
			}
			if haveStartUsage {
				t.startUsageBytes = new(float64)
			}
			if haveCurrentUsage {
				t.currentUsageBytes = new(float64)
			}
			if haveUsageStats {
				t.peakUsageBytes = new(float64)
				t.avgUsageBytes = new(float64)
			}
		}
	}
	return placement
}

func (t *podTotals) add(pod controlPlanePod) {
	t.pods++
	t.memoryRequestBytes += pod.memoryRequestBytes
	addSample(&t.startUsageBytes, pod.startUsageBytes)
	addSample(&t.currentUsageBytes, pod.currentUsageBytes)
	addSample(&t.peakUsageBytes, pod.peakUsageBytes)
	addSample(&t.avgUsageBytes, pod.avgUsageBytes)
}

// setRow writes the totals to columns named with the given prefix, e.g. SingletonPods.
func (t *podTotals) setRow(row map[string]string, prefix string) {
	row[prefix+"Pods"] = fmt.Sprintf("%d", t.pods)
	row[prefix+"MemoryRequestMB"] = formatMB(float64(t.memoryRequestBytes))
	setMB(row, prefix+"StartMemoryUsageMB", t.startUsageBytes)
	setMB(row, prefix+"EndMemoryUsageMB", t.currentUsageBytes)
	setMB(row, prefix+"PeakMemoryUsageMB", t.peakUsageBytes)
	setMB(row, prefix+"AvgMemoryUsageMB", t.avgUsageBytes)
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
// with the number of pods. Peak and average are distinguished by a "stat" label because
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
		logger.Info("No control plane pod placement data to export")
		return nil
	}

	podRows := make([]map[string]string, 0, len(w.pods))
	for _, pod := range w.pods {
		row := map[string]string{
			"Namespace":       pod.namespace,
			"Deployment":      pod.workload,
			"WorkloadKind":    pod.workloadKind,
			"PodName":         pod.podName,
			"NodeName":        pod.nodeName,
			"MemoryRequestMB": formatMB(float64(pod.memoryRequestBytes)),
			"PodAgeSeconds":   fmt.Sprintf("%d", int64(pod.podAge.Seconds())),
		}
		if pod.replicas != nil {
			row["Replicas"] = fmt.Sprintf("%d", *pod.replicas)
		}
		setMB(row, "StartMemoryUsageMB", pod.startUsageBytes)
		setMB(row, "EndMemoryUsageMB", pod.currentUsageBytes)
		setMB(row, "PeakMemoryUsageMB", pod.peakUsageBytes)
		setMB(row, "AvgMemoryUsageMB", pod.avgUsageBytes)
		podRows = append(podRows, row)
	}
	// The table and the Deployment column predate covering all control plane pods. Deployment holds
	// the workload name for every WorkloadKind; the original singleton rows are those with
	// WorkloadKind = 'Deployment' AND Replicas = 1 (rows loaded before WorkloadKind existed are all
	// singletons).
	podsFile := dataloader.DataFile{
		TableName: "control_plane_singleton_pods",
		Schema: map[string]dataloader.DataType{
			"Namespace":          dataloader.DataTypeString,
			"Deployment":         dataloader.DataTypeString,
			"WorkloadKind":       dataloader.DataTypeString,
			"Replicas":           dataloader.DataTypeInteger,
			"PodName":            dataloader.DataTypeString,
			"NodeName":           dataloader.DataTypeString,
			"MemoryRequestMB":    dataloader.DataTypeFloat64,
			"StartMemoryUsageMB": dataloader.DataTypeFloat64,
			"EndMemoryUsageMB":   dataloader.DataTypeFloat64,
			"PeakMemoryUsageMB":  dataloader.DataTypeFloat64,
			"AvgMemoryUsageMB":   dataloader.DataTypeFloat64,
			"PodAgeSeconds":      dataloader.DataTypeInteger,
		},
		Rows: podRows,
	}

	placementRows := make([]map[string]string, 0, len(w.placement))
	for _, p := range w.placement {
		row := map[string]string{
			"NodeName":     p.nodeName,
			"TotalMasters": fmt.Sprintf("%d", len(w.placement)),
		}
		p.singletons.setRow(row, "Singleton")
		p.all.setRow(row, "All")
		placementRows = append(placementRows, row)
	}
	// Singleton* columns keep their original single-replica Deployment meaning; All* columns cover
	// every control plane pod on the master and are NULL for rows loaded before they were added.
	placementSchema := map[string]dataloader.DataType{
		"NodeName":     dataloader.DataTypeString,
		"TotalMasters": dataloader.DataTypeInteger,
	}
	for _, prefix := range []string{"Singleton", "All"} {
		placementSchema[prefix+"Pods"] = dataloader.DataTypeInteger
		for _, column := range []string{"MemoryRequestMB", "StartMemoryUsageMB", "EndMemoryUsageMB", "PeakMemoryUsageMB", "AvgMemoryUsageMB"} {
			placementSchema[prefix+column] = dataloader.DataTypeFloat64
		}
	}
	placementFile := dataloader.DataFile{
		TableName: "control_plane_singleton_placement",
		Schema:    placementSchema,
		Rows:      placementRows,
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
