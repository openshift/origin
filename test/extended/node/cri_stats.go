package node

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	k8simage "k8s.io/kubernetes/test/utils/image"
	"k8s.io/utils/ptr"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	criStatsContainerName = "busybox-container"
	criStatsMemoryLimit   = "80Mi"

	criStatsKi = 1024
	criStatsMi = 1024 * criStatsKi
)

// criStatsPodNames mirrors the pod naming of the upstream e2e_node test
// (kubernetes/kubernetes test/e2e_node/cri_container_metrics_test.go).
var criStatsPodNames = []string{"stats-busybox-0", "stats-busybox-1"}

// When PodAndContainerStatsFromCRI is enabled, kubelet collects pod and container
// stats from CRI-O through the CRI stats API instead of cAdvisor, and CRI-O becomes
// the single source of truth for metrics exposed on /metrics/cadvisor.
//
// The [OCPFeatureGate:PodAndContainerStatsFromCRI] label skips this test on clusters
// where the gate is disabled (e.g. the Default feature set), so it only runs on
// TechPreviewNoUpgrade/DevPreviewNoUpgrade clusters (and CustomNoUpgrade clusters
// that enabled the gate explicitly).
var _ = g.Describe("[sig-node][OCPFeatureGate:PodAndContainerStatsFromCRI][Jira:Node][Suite:openshift/conformance/parallel][Skipped:Disconnected] CRI pod and container stats", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("cri-stats")

	g.It("should report pod and container metrics from CRI on /metrics/cadvisor [Timeout:15m]", func(ctx context.Context) {
		// The [OCPFeatureGate:PodAndContainerStatsFromCRI] label skips this test on
		// clusters where the gate is disabled; double-check the reported gate status
		// here so CustomNoUpgrade clusters that did not enable the gate also skip.
		SkipOnMicroShift(oc)
		if enabled, reason := isCriStatsEnabled(ctx, oc); !enabled {
			g.Skip(reason)
		}

		g.By("Verifying kubelet on the worker node runs with PodAndContainerStatsFromCRI enabled")
		nodeName := GetFirstReadyWorkerNode(oc)
		kubeletConfig, err := getKubeletConfigFromNode(ctx, oc, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read kubelet config from node %s", nodeName)
		o.Expect(kubeletConfig.FeatureGates).To(o.HaveKeyWithValue("PodAndContainerStatsFromCRI", true),
			"kubelet on node %s is not running with PodAndContainerStatsFromCRI enabled", nodeName)

		g.By("Creating a test namespace with pods pinned to the node")
		testNamespace, err := e2e.CreateTestingNS(ctx, "cri-stats", oc.AdminKubeClient(), map[string]string{
			"pod-security.kubernetes.io/enforce": "baseline",
			"pod-security.kubernetes.io/audit":   "baseline",
			"pod-security.kubernetes.io/warn":    "baseline",
		})
		// CreateTestingNS can return the namespace together with an error, e.g.
		// when the default service account is slow to appear; register the
		// cleanup before failing on the error so the namespace does not leak.
		if testNamespace != nil {
			g.DeferCleanup(criStatsDeleteNamespace, oc, testNamespace.Name)
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create test namespace")
		namespace := testNamespace.Name

		// All test containers start after metricsSince; capture it once so the
		// lower bound for start-time metrics cannot advance past their start
		// times while Eventually and Consistently retry.
		metricsSince := time.Now()

		for _, podName := range criStatsPodNames {
			pod := criStatsTestPod(namespace, podName, nodeName)
			_, err := oc.AdminKubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to create pod %s", podName)
			err = e2epod.WaitForPodRunningInNamespace(ctx, oc.AdminKubeClient(), pod)
			o.Expect(err).NotTo(o.HaveOccurred(), "pod %s failed to start", podName)
		}

		g.By("Waiting for container and pod metrics from CRI to be reported on /metrics/cadvisor")
		o.Eventually(ctx, func(ctx context.Context) error {
			return criStatsValidateMetrics(ctx, oc, nodeName, namespace, metricsSince)
		}, 5*time.Minute, 30*time.Second).Should(o.Succeed())

		g.By("Ensuring the reported metrics stay within the expected ranges")
		o.Consistently(ctx, func(ctx context.Context) error {
			return criStatsValidateMetrics(ctx, oc, nodeName, namespace, metricsSince)
		}, 1*time.Minute, 15*time.Second).Should(o.Succeed())

		g.By("Checking kubelet exposes kubelet_metrics_provider with provider=cri")
		o.Eventually(ctx, func(ctx context.Context) error {
			return criStatsCheckMetricsProvider(ctx, oc, nodeName)
		}, 2*time.Minute, 15*time.Second).Should(o.Succeed())
	})
})

// isCriStatsEnabled checks whether the PodAndContainerStatsFromCRI feature gate
// is enabled on the cluster.
func isCriStatsEnabled(ctx context.Context, oc *exutil.CLI) (bool, string) {
	fgs, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get FeatureGate resource")
	for _, fg := range fgs.Status.FeatureGates {
		for _, enabledFG := range fg.Enabled {
			if enabledFG.Name == "PodAndContainerStatsFromCRI" {
				return true, ""
			}
		}
	}
	return false, "PodAndContainerStatsFromCRI feature gate is not enabled"
}

// criStatsDeleteNamespace deletes the test namespace. The bounded context keeps
// the cleanup from hanging indefinitely when the API server is unreachable.
func criStatsDeleteNamespace(oc *exutil.CLI, name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := oc.AdminKubeClient().CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// criStatsTestPod returns a small pod whose resource usage is asserted through the
// CRI-sourced kubelet metrics.
func criStatsTestPod(namespace, name, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			NodeName:      nodeName,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:      ptr.To[int64](1000),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{
				{
					Name:    criStatsContainerName,
					Image:   k8simage.GetE2EImage(k8simage.BusyBox),
					Command: []string{"sh", "-c", "echo running && sleep 3600"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse(criStatsMemoryLimit),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
					},
				},
			},
		},
	}
}

// criStatsMetricRange describes the expected [min, max] range of a metric.
type criStatsMetricRange struct {
	name string
	min  float64
	max  float64
}

// criStatsRequiredContainerMetrics must be reported by CRI-O for every test
// container, with values matching the upstream e2e_node expectations.
func criStatsRequiredContainerMetrics(recentLower, futureUpper float64) []criStatsMetricRange {
	return []criStatsMetricRange{
		{"container_cpu_system_seconds_total", 0, 100},
		{"container_cpu_usage_seconds_total", 0, 100},
		{"container_cpu_user_seconds_total", 0, 100},
		{"container_file_descriptors", 0, 100},
		{"container_fs_reads_bytes_total", 0, 10000000},
		{"container_fs_reads_total", 0, 100},
		{"container_fs_writes_bytes_total", 0, 1000000},
		{"container_fs_writes_total", 0, 200},
		{"container_last_seen", recentLower, futureUpper},
		{"container_memory_cache", 0, 10 * criStatsMi},
		{"container_memory_failures_total", 0, 1000000},
		{"container_memory_mapped_file", 0, 10000000},
		{"container_memory_max_usage_bytes", 0, 80 * criStatsMi},
		{"container_memory_rss", 10 * criStatsKi, 80 * criStatsMi},
		// Unlike upstream, swap usage is only bounded and not required to be zero
		// because OpenShift supports nodes with swap enabled.
		{"container_memory_swap", 0, 80 * criStatsMi},
		{"container_memory_usage_bytes", 10 * criStatsKi, 80 * criStatsMi},
		{"container_memory_working_set_bytes", 10 * criStatsKi, 80 * criStatsMi},
		{"container_oom_events_total", 0, 0},
		{"container_processes", 0, 10},
		{"container_sockets", 0, 10},
		{"container_spec_cpu_period", 100000, 100000},
		{"container_spec_cpu_shares", 2, 2},
		// CRI runtimes may report the exact memory limit (80Mi) without the
		// page alignment cAdvisor applies, so accept both.
		{"container_spec_memory_limit_bytes", 79953920, 80 * criStatsMi},
		{"container_spec_memory_swap_limit_bytes", 0, 80 * criStatsMi},
		{"container_start_time_seconds", recentLower, futureUpper},
		{"container_threads", 0, 10},
		// CRI runtimes report the kernel pids.max, which is MaxUint64 when no
		// limit is set.
		{"container_threads_max", 0, 1.85e19},
	}
}

// criStatsNetworkMetrics are reported once per pod on the sandbox container.
func criStatsNetworkMetrics() []criStatsMetricRange {
	return []criStatsMetricRange{
		{"container_network_receive_bytes_total", 0, 10 * criStatsMi},
		{"container_network_receive_errors_total", 0, 1000},
		{"container_network_receive_packets_dropped_total", 0, 1000},
		{"container_network_receive_packets_total", 0, 1000},
		{"container_network_transmit_bytes_total", 0, 10 * criStatsMi},
		{"container_network_transmit_errors_total", 0, 1000},
		{"container_network_transmit_packets_dropped_total", 0, 1000},
		{"container_network_transmit_packets_total", 0, 1000},
	}
}

// criStatsOptionalContainerMetrics may be absent depending on runtime or kernel
// support; when they are reported their values must be sane. The fs metrics are
// filesystem/partition-scoped in CRI rather than per-container, so their bounds
// are deliberately loose.
func criStatsOptionalContainerMetrics() []criStatsMetricRange {
	return []criStatsMetricRange{
		{"container_blkio_device_usage_total", 0, 1e12},
		{"container_cpu_cfs_periods_total", 0, 1e9},
		{"container_cpu_cfs_throttled_periods_total", 0, 1e9},
		{"container_cpu_cfs_throttled_seconds_total", 0, 1e6},
		{"container_fs_limit_bytes", 0, 1e15},
		{"container_fs_usage_bytes", 0, 1e15},
		{"container_fs_inodes_free", 0, 1e12},
		{"container_fs_inodes_total", 0, 1e12},
		{"container_hugetlb_max_usage_bytes", 0, 80 * criStatsMi},
		{"container_hugetlb_usage_bytes", 0, 80 * criStatsMi},
		{"container_pressure_cpu_stalled_seconds_total", 0, 1e6},
		{"container_pressure_cpu_waiting_seconds_total", 0, 1e6},
		{"container_pressure_io_stalled_seconds_total", 0, 1e6},
		{"container_pressure_io_waiting_seconds_total", 0, 1e6},
		{"container_pressure_memory_stalled_seconds_total", 0, 1e6},
		{"container_pressure_memory_waiting_seconds_total", 0, 1e6},
		{"container_spec_cpu_quota", 0, 1e6},
		{"container_spec_memory_reservation_limit_bytes", 0, 80 * criStatsMi},
		{"container_ulimits_soft", 0, 1073741816},
	}
}

// criStatsValidateMetrics scrapes /metrics/cadvisor on the node and verifies the
// CRI-sourced metrics of the test pods match the expected ranges. All test
// containers started after metricsSince; the lower bound is derived from it so
// it stays fixed across Eventually and Consistently retries.
func criStatsValidateMetrics(ctx context.Context, oc *exutil.CLI, nodeName, namespace string, metricsSince time.Time) error {
	families, err := criStatsScrape(ctx, oc, nodeName, "metrics/cadvisor")
	if err != nil {
		return err
	}

	// The margin below metricsSince tolerates clock skew between the test
	// runner and the node.
	recentLower := float64(metricsSince.Add(-2 * time.Minute).Unix())
	futureUpper := float64(time.Now().Add(2 * time.Minute).Unix())

	var errs []error
	for _, podName := range criStatsPodNames {
		containerLabels := map[string]string{
			"namespace": namespace,
			"pod":       podName,
			"container": criStatsContainerName,
		}
		for _, r := range criStatsRequiredContainerMetrics(recentLower, futureUpper) {
			if err := criStatsCheckMetric(families, r, containerLabels); err != nil {
				errs = append(errs, fmt.Errorf("pod %s: %w", podName, err))
			}
		}
		for _, r := range criStatsOptionalContainerMetrics() {
			if err := criStatsCheckOptionalMetric(families, r, containerLabels); err != nil {
				errs = append(errs, fmt.Errorf("pod %s: %w", podName, err))
			}
		}

		// Network metrics only exist once per pod, on the sandbox. Runtimes
		// disagree on the sandbox's container label (CRI-O and cAdvisor use
		// "POD", containerd leaves it empty), so match on namespace and pod
		// only.
		podLabels := map[string]string{"namespace": namespace, "pod": podName}
		for _, r := range criStatsNetworkMetrics() {
			if err := criStatsCheckMetric(families, r, podLabels); err != nil {
				errs = append(errs, fmt.Errorf("pod %s: %w", podName, err))
			}
		}
	}
	return errors.NewAggregate(errs)
}

// criStatsCheckMetricsProvider verifies kubelet reports kubelet_metrics_provider
// with provider=cri, i.e. stats are collected from the runtime instead of cAdvisor.
func criStatsCheckMetricsProvider(ctx context.Context, oc *exutil.CLI, nodeName string) error {
	families, err := criStatsScrape(ctx, oc, nodeName, "metrics")
	if err != nil {
		return err
	}
	family, ok := families["kubelet_metrics_provider"]
	if !ok {
		return fmt.Errorf("kubelet_metrics_provider metric not reported by kubelet on node %s", nodeName)
	}
	for _, sample := range family.Metric {
		if criStatsLabelValue(sample, "provider") == "cri" {
			return nil
		}
	}
	return fmt.Errorf("kubelet_metrics_provider with provider=cri not reported by kubelet on node %s", nodeName)
}

// criStatsScrape reads a kubelet endpoint on the given node through the API
// server node proxy, e.g. metrics or metrics/cadvisor.
func criStatsScrape(ctx context.Context, oc *exutil.CLI, nodeName, path string) (map[string]*dto.MetricFamily, error) {
	raw, err := oc.AdminKubeClient().CoreV1().RESTClient().Get().
		AbsPath(fmt.Sprintf("/api/v1/nodes/%s/proxy/%s", nodeName, path)).DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape kubelet /%s on node %s: %w", path, nodeName, err)
	}
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewBuffer(raw))
	if err != nil {
		return nil, fmt.Errorf("failed to parse kubelet /%s output on node %s: %w", path, nodeName, err)
	}
	return families, nil
}

func criStatsCheckMetric(families map[string]*dto.MetricFamily, expected criStatsMetricRange, labels map[string]string) error {
	values := criStatsSampleValues(families, expected.name, labels)
	if len(values) == 0 {
		return fmt.Errorf("required metric %s with labels %v not reported", expected.name, labels)
	}
	return criStatsCheckValues(expected, values, labels)
}

func criStatsCheckOptionalMetric(families map[string]*dto.MetricFamily, expected criStatsMetricRange, labels map[string]string) error {
	values := criStatsSampleValues(families, expected.name, labels)
	if len(values) == 0 {
		return nil
	}
	return criStatsCheckValues(expected, values, labels)
}

func criStatsCheckValues(expected criStatsMetricRange, values []float64, labels map[string]string) error {
	for _, v := range values {
		// NaN compares false against both bounds and would otherwise pass.
		if math.IsNaN(v) || v < expected.min || v > expected.max {
			return fmt.Errorf("metric %s with labels %v reported %v, expected within [%v, %v]",
				expected.name, labels, v, expected.min, expected.max)
		}
	}
	return nil
}

// criStatsSampleValues returns the values of all samples of the given metric
// whose labels contain the given label set.
func criStatsSampleValues(families map[string]*dto.MetricFamily, metric string, labels map[string]string) []float64 {
	family, ok := families[metric]
	if !ok {
		return nil
	}
	var values []float64
	for _, sample := range family.Metric {
		if !criStatsHasLabels(sample, labels) {
			continue
		}
		if v, ok := criStatsSampleValue(sample); ok {
			values = append(values, v)
		}
	}
	return values
}

func criStatsHasLabels(sample *dto.Metric, labels map[string]string) bool {
	for name, value := range labels {
		if criStatsLabelValue(sample, name) != value {
			return false
		}
	}
	return true
}

func criStatsLabelValue(sample *dto.Metric, name string) string {
	for _, label := range sample.GetLabel() {
		if label.GetName() == name {
			return label.GetValue()
		}
	}
	return ""
}

// criStatsSampleValue extracts the sample value regardless of the metric type.
func criStatsSampleValue(sample *dto.Metric) (float64, bool) {
	switch {
	case sample.GetGauge() != nil:
		return sample.GetGauge().GetValue(), true
	case sample.GetCounter() != nil:
		return sample.GetCounter().GetValue(), true
	case sample.GetUntyped() != nil:
		return sample.GetUntyped().GetValue(), true
	default:
		return 0, false
	}
}
