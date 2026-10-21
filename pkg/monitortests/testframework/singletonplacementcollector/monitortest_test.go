package singletonplacementcollector

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/dataloader"
	prometheustypes "github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"k8s.io/utils/ptr"
)

var (
	testNow       = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	testBeginning = testNow.Add(-2 * time.Hour)
)

func node(name string, roles ...string) *corev1.Node {
	labels := map[string]string{}
	for _, role := range roles {
		labels["node-role.kubernetes.io/"+role] = ""
	}
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

func threeMasters() []runtime.Object {
	return []runtime.Object{node("master-0", "master"), node("master-1", "master"), node("master-2", "control-plane"), node("worker-0", "worker")}
}

// singleton creates a Deployment, its ReplicaSet and one Running pod with the given memory
// requests per container, created an hour before testNow.
func singleton(namespace, name, nodeName string, replicas int32, memoryRequests ...string) []runtime.Object {
	rsName := name + "-abc123"
	var containers []corev1.Container
	for _, req := range memoryRequests {
		containers = append(containers, corev1.Container{
			Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse(req)}},
		})
	}
	return []runtime.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec:       appsv1.DeploymentSpec{Replicas: ptr.To(replicas)},
		},
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
			Namespace:       namespace,
			Name:            rsName,
			OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: name, Controller: ptr.To(true)}},
		}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:         namespace,
				Name:              rsName + "-xyz",
				CreationTimestamp: metav1.NewTime(testNow.Add(-time.Hour)),
				OwnerReferences:   []metav1.OwnerReference{{Kind: "ReplicaSet", Name: rsName, Controller: ptr.To(true)}},
			},
			Spec:   corev1.PodSpec{NodeName: nodeName, Containers: containers},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
	}
}

func withPhase(objs []runtime.Object, phase corev1.PodPhase) []runtime.Object {
	objs[2].(*corev1.Pod).Status.Phase = phase
	return objs
}

// replaced moves the singleton's pod to a new name and node, as if it was rescheduled.
func replaced(objs []runtime.Object, podName, nodeName string) []runtime.Object {
	pod := objs[2].(*corev1.Pod)
	pod.Name = podName
	pod.Spec.NodeName = nodeName
	return objs
}

func objects(groups ...[]runtime.Object) []runtime.Object {
	var ret []runtime.Object
	for _, g := range groups {
		ret = append(ret, g...)
	}
	return ret
}

func newTestCollector(currentUsage map[string]float64, stats map[string]usageStats) *singletonPlacementCollector {
	return &singletonPlacementCollector{
		fetchCurrentUsage: func(context.Context, kubernetes.Interface) (map[string]float64, error) {
			if currentUsage == nil {
				return nil, errors.New("metrics API unavailable")
			}
			return currentUsage, nil
		},
		fetchUsageStats: func(context.Context, *rest.Config, kubernetes.Interface, time.Time, time.Time) (map[string]usageStats, error) {
			if stats == nil {
				return nil, errors.New("prometheus unavailable")
			}
			return stats, nil
		},
	}
}

func TestCollectPlacement(t *testing.T) {
	testCases := []struct {
		name string
		objs []runtime.Object
		// expected singleton count per master; nil means nothing should be recorded
		expectedPerMaster map[string]int
		expectedPods      []string
	}{
		{
			name: "skewed placement",
			objs: objects(threeMasters(),
				singleton("openshift-a", "a", "master-0", 1, "50Mi"),
				singleton("openshift-b", "b", "master-0", 1, "50Mi"),
				singleton("openshift-c", "c", "master-0", 1, "50Mi"),
				singleton("openshift-d", "d", "master-1", 1, "50Mi"),
			),
			expectedPerMaster: map[string]int{"master-0": 3, "master-1": 1, "master-2": 0},
			expectedPods:      []string{"a-abc123-xyz", "b-abc123-xyz", "c-abc123-xyz", "d-abc123-xyz"},
		},
		{
			name: "balanced placement",
			objs: objects(threeMasters(),
				singleton("openshift-a", "a", "master-0", 1, "50Mi"),
				singleton("openshift-b", "b", "master-1", 1, "50Mi"),
				singleton("openshift-c", "c", "master-2", 1, "50Mi"),
			),
			expectedPerMaster: map[string]int{"master-0": 1, "master-1": 1, "master-2": 1},
			expectedPods:      []string{"a-abc123-xyz", "b-abc123-xyz", "c-abc123-xyz"},
		},
		{
			name:              "masters with zero singletons still get rows",
			objs:              objects(threeMasters()),
			expectedPerMaster: map[string]int{"master-0": 0, "master-1": 0, "master-2": 0},
		},
		{
			name: "multi-replica deployments excluded",
			objs: objects(threeMasters(),
				singleton("openshift-a", "a", "master-0", 1, "50Mi"),
				singleton("openshift-b", "b", "master-0", 3, "50Mi"),
			),
			expectedPerMaster: map[string]int{"master-0": 1, "master-1": 0, "master-2": 0},
			expectedPods:      []string{"a-abc123-xyz"},
		},
		{
			name: "non-openshift namespaces excluded",
			objs: objects(threeMasters(),
				singleton("openshift-a", "a", "master-0", 1, "50Mi"),
				singleton("kube-system", "b", "master-0", 1, "50Mi"),
				singleton("e2e-test-foo", "c", "master-1", 1, "50Mi"),
			),
			expectedPerMaster: map[string]int{"master-0": 1, "master-1": 0, "master-2": 0},
			expectedPods:      []string{"a-abc123-xyz"},
		},
		{
			name: "worker-only pods excluded",
			objs: objects(threeMasters(),
				singleton("openshift-a", "a", "master-0", 1, "50Mi"),
				singleton("openshift-b", "b", "worker-0", 1, "50Mi"),
			),
			expectedPerMaster: map[string]int{"master-0": 1, "master-1": 0, "master-2": 0},
			expectedPods:      []string{"a-abc123-xyz"},
		},
		{
			name: "pods that are not running excluded",
			objs: objects(threeMasters(),
				singleton("openshift-a", "a", "master-0", 1, "50Mi"),
				withPhase(singleton("openshift-b", "b", "master-1", 1, "50Mi"), corev1.PodPending),
			),
			expectedPerMaster: map[string]int{"master-0": 1, "master-1": 0, "master-2": 0},
			expectedPods:      []string{"a-abc123-xyz"},
		},
		{
			name: "single master records nothing",
			objs: objects(
				[]runtime.Object{node("master-0", "master"), node("worker-0", "worker")},
				singleton("openshift-a", "a", "master-0", 1, "50Mi"),
			),
		},
		{
			name: "no masters records nothing",
			objs: objects(
				[]runtime.Object{node("worker-0", "worker"), node("worker-1", "worker")},
				singleton("openshift-a", "a", "worker-0", 1, "50Mi"),
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestCollector(map[string]float64{}, map[string]usageStats{})
			require.NoError(t, w.collectFromClient(context.Background(), fake.NewSimpleClientset(tc.objs...), testBeginning, testNow, testNow))

			if tc.expectedPerMaster == nil {
				assert.Empty(t, w.placement)
				assert.Empty(t, w.pods)
				return
			}

			actualPerMaster := map[string]int{}
			for _, p := range w.placement {
				actualPerMaster[p.nodeName] = p.singletonPods
			}
			assert.Equal(t, tc.expectedPerMaster, actualPerMaster)

			var actualPods []string
			for _, p := range w.pods {
				actualPods = append(actualPods, p.podName)
			}
			sort.Strings(actualPods)
			assert.Equal(t, tc.expectedPods, actualPods)
		})
	}
}

func TestCollectPodDetailsAndUsage(t *testing.T) {
	objs := objects(threeMasters(),
		singleton("openshift-a", "a", "master-0", 1, "50Mi", "30Mi"),
		singleton("openshift-b", "b", "master-0", 1, "100Mi"),
	)
	mb := func(v float64) float64 { return v * bytesPerMB }

	testCases := []struct {
		name         string
		currentUsage map[string]float64
		stats        map[string]usageStats

		expectedMaster0Current *float64
		expectedMaster0Peak    *float64
		expectedMaster0Avg     *float64
		expectedMaster1Current *float64
		expectedPodBPeak       *float64
	}{
		{
			name: "usage available",
			currentUsage: map[string]float64{
				"openshift-a/a-abc123-xyz": mb(200),
				"openshift-b/b-abc123-xyz": mb(150),
			},
			stats: map[string]usageStats{
				"openshift-a/a-abc123-xyz": {peakBytes: mb(300), avgBytes: mb(180)},
				"openshift-b/b-abc123-xyz": {peakBytes: mb(250), avgBytes: mb(140)},
			},
			expectedMaster0Current: ptr.To(mb(350)),
			expectedMaster0Peak:    ptr.To(mb(550)),
			expectedMaster0Avg:     ptr.To(mb(320)),
			expectedMaster1Current: ptr.To(0.0),
			expectedPodBPeak:       ptr.To(mb(250)),
		},
		{
			name: "pod missing from metrics is left unset and excluded from sums",
			currentUsage: map[string]float64{
				"openshift-a/a-abc123-xyz": mb(200),
			},
			stats: map[string]usageStats{
				"openshift-a/a-abc123-xyz": {peakBytes: mb(300), avgBytes: mb(180)},
			},
			expectedMaster0Current: ptr.To(mb(200)),
			expectedMaster0Peak:    ptr.To(mb(300)),
			expectedMaster0Avg:     ptr.To(mb(180)),
			expectedMaster1Current: ptr.To(0.0),
		},
		{
			name: "metric sources unavailable leaves usage unset",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestCollector(tc.currentUsage, tc.stats)
			require.NoError(t, w.collectFromClient(context.Background(), fake.NewSimpleClientset(objs...), testBeginning, testNow, testNow))

			require.Len(t, w.pods, 2)
			sort.Slice(w.pods, func(i, j int) bool { return w.pods[i].podName < w.pods[j].podName })
			assert.Equal(t, "a", w.pods[0].deployment)
			assert.Equal(t, int64(80*bytesPerMB), w.pods[0].memoryRequestBytes)
			assert.Equal(t, time.Hour, w.pods[0].podAge)
			assert.Equal(t, tc.expectedPodBPeak, w.pods[1].peakUsageBytes)

			require.Len(t, w.placement, 3)
			assert.Equal(t, "master-0", w.placement[0].nodeName)
			assert.Equal(t, 2, w.placement[0].singletonPods)
			assert.Equal(t, int64(180*bytesPerMB), w.placement[0].memoryRequestBytes)
			assert.Equal(t, tc.expectedMaster0Current, w.placement[0].currentUsageBytes)
			assert.Equal(t, tc.expectedMaster0Peak, w.placement[0].peakUsageBytes)
			assert.Equal(t, tc.expectedMaster0Avg, w.placement[0].avgUsageBytes)
			assert.Equal(t, tc.expectedMaster1Current, w.placement[1].currentUsageBytes)
		})
	}
}

func TestCollectStartUsage(t *testing.T) {
	mb := func(v float64) float64 { return v * bytesPerMB }
	startObjs := objects(threeMasters(),
		singleton("openshift-a", "a", "master-0", 1, "50Mi"),
		singleton("openshift-b", "b", "master-0", 1, "50Mi"),
	)
	endObjs := objects(threeMasters(),
		singleton("openshift-a", "a", "master-0", 1, "50Mi"),
		replaced(singleton("openshift-b", "b", "master-0", 1, "50Mi"), "b-abc123-new", "master-1"),
		singleton("openshift-c", "c", "master-1", 1, "50Mi"),
	)

	testCases := []struct {
		name       string
		startUsage map[string]float64

		// expected start usage per end pod name, and per master in master order
		expectedPods      map[string]*float64
		expectedPlacement []*float64
	}{
		{
			name: "start usage joined by deployment",
			startUsage: map[string]float64{
				"openshift-a/a-abc123-xyz": mb(100),
				"openshift-b/b-abc123-xyz": mb(40),
			},
			expectedPods: map[string]*float64{
				"a-abc123-xyz": ptr.To(mb(100)),
				// replaced pod keeps its deployment's start value
				"b-abc123-new": ptr.To(mb(40)),
				// deployment created during the run has no start value
				"c-abc123-xyz": nil,
			},
			expectedPlacement: []*float64{ptr.To(mb(100)), ptr.To(mb(40)), ptr.To(0.0)},
		},
		{
			name: "metrics API unavailable at start leaves start usage unset",
			expectedPods: map[string]*float64{
				"a-abc123-xyz": nil,
				"b-abc123-new": nil,
				"c-abc123-xyz": nil,
			},
			expectedPlacement: []*float64{nil, nil, nil},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestCollector(tc.startUsage, map[string]usageStats{})
			require.NoError(t, w.collectStartUsage(context.Background(), fake.NewSimpleClientset(startObjs...), testBeginning))

			w.fetchCurrentUsage = func(context.Context, kubernetes.Interface) (map[string]float64, error) {
				return map[string]float64{}, nil
			}
			require.NoError(t, w.collectFromClient(context.Background(), fake.NewSimpleClientset(endObjs...), testBeginning, testNow, testNow))

			actualPods := map[string]*float64{}
			for _, p := range w.pods {
				actualPods[p.podName] = p.startUsageBytes
			}
			assert.Equal(t, tc.expectedPods, actualPods)

			var actualPlacement []*float64
			for _, p := range w.placement {
				actualPlacement = append(actualPlacement, p.startUsageBytes)
			}
			assert.Equal(t, tc.expectedPlacement, actualPlacement)
		})
	}
}

func TestSummarizePlacementUsage(t *testing.T) {
	masters := []string{"master-0", "master-1", "master-2"}

	testCases := []struct {
		name             string
		pods             []singletonPod
		haveCurrentUsage bool
		// expected currentUsageBytes per master, in master order
		expected []*float64
	}{
		{
			name: "partial samples on a master are summed",
			pods: []singletonPod{
				{nodeName: "master-0", currentUsageBytes: ptr.To(100.0)},
				{nodeName: "master-0"},
				{nodeName: "master-1", currentUsageBytes: ptr.To(50.0)},
			},
			haveCurrentUsage: true,
			expected:         []*float64{ptr.To(100.0), ptr.To(50.0), ptr.To(0.0)},
		},
		{
			name: "master whose singletons all lack samples is left unset",
			pods: []singletonPod{
				{nodeName: "master-0", currentUsageBytes: ptr.To(100.0)},
				{nodeName: "master-1"},
				{nodeName: "master-1"},
			},
			haveCurrentUsage: true,
			expected:         []*float64{ptr.To(100.0), nil, ptr.To(0.0)},
		},
		{
			name: "source without data leaves every master unset",
			pods: []singletonPod{
				{nodeName: "master-0"},
			},
			haveCurrentUsage: false,
			expected:         []*float64{nil, nil, nil},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			placement := summarizePlacement(masters, tc.pods, false, tc.haveCurrentUsage, false)
			var actual []*float64
			for _, p := range placement {
				actual = append(actual, p.currentUsageBytes)
				assert.Nil(t, p.startUsageBytes)
				assert.Nil(t, p.peakUsageBytes)
				assert.Nil(t, p.avgUsageBytes)
			}
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestUsageStatsFromQueryResult(t *testing.T) {
	sample := func(namespace, pod, stat string, v float64) *prometheustypes.Sample {
		return &prometheustypes.Sample{
			Metric: prometheustypes.Metric{"namespace": prometheustypes.LabelValue(namespace), "pod": prometheustypes.LabelValue(pod), "stat": prometheustypes.LabelValue(stat)},
			Value:  prometheustypes.SampleValue(v),
		}
	}
	result := prometheustypes.Vector{
		sample("openshift-a", "a-1", "peak", 300),
		sample("openshift-a", "a-1", "avg", 200),
		sample("openshift-b", "b-1", "avg", 50),
		sample("openshift-b", "b-1", "peak", 90),
	}

	stats, err := usageStatsFromQueryResult(result)
	require.NoError(t, err)
	assert.Equal(t, map[string]usageStats{
		"openshift-a/a-1": {peakBytes: 300, avgBytes: 200},
		"openshift-b/b-1": {peakBytes: 90, avgBytes: 50},
	}, stats)

	_, err = usageStatsFromQueryResult(prometheustypes.Matrix{})
	assert.Error(t, err)
}

func TestUsageStatsQuery(t *testing.T) {
	query := usageStatsQuery(90 * time.Minute)
	assert.Contains(t, query, `max_over_time(sum by (namespace, pod) (container_memory_working_set_bytes{`)
	assert.Contains(t, query, `avg_over_time(sum by (namespace, pod) (container_memory_working_set_bytes{`)
	assert.Contains(t, query, "[5400s:1m]")
}

func TestCurrentUsageFromPodMetrics(t *testing.T) {
	podMetrics := &metricsv1beta1.PodMetricsList{Items: []metricsv1beta1.PodMetrics{{
		ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-a", Name: "a-1"},
		Containers: []metricsv1beta1.ContainerMetrics{
			{Usage: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("100Mi")}},
			{Usage: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("20Mi")}},
		},
	}}}
	assert.Equal(t, map[string]float64{"openshift-a/a-1": 120 * bytesPerMB}, currentUsageFromPodMetrics(podMetrics))
}

func TestWriteContentToStorage(t *testing.T) {
	t.Run("nothing written without placement data", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, (&singletonPlacementCollector{}).WriteContentToStorage(context.Background(), dir, "_suffix", nil, nil))
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("pods and placement tables written", func(t *testing.T) {
		dir := t.TempDir()
		w := &singletonPlacementCollector{
			pods: []singletonPod{{
				namespace: "openshift-a", deployment: "a", podName: "a-1", nodeName: "master-0",
				memoryRequestBytes: 50 * bytesPerMB, podAge: 90 * time.Second,
				startUsageBytes: ptr.To(120.0 * bytesPerMB), currentUsageBytes: ptr.To(250.0 * bytesPerMB),
				peakUsageBytes: ptr.To(300.0 * bytesPerMB), avgUsageBytes: ptr.To(200.0 * bytesPerMB),
			}},
			placement: []masterPlacement{
				{nodeName: "master-0", singletonPods: 1, memoryRequestBytes: 50 * bytesPerMB, startUsageBytes: ptr.To(120.0 * bytesPerMB), currentUsageBytes: ptr.To(250.0 * bytesPerMB)},
				{nodeName: "master-1"},
			},
		}
		require.NoError(t, w.WriteContentToStorage(context.Background(), dir, "_suffix", nil, nil))

		pods := readDataFile(t, filepath.Join(dir, "control-plane-singleton-pods_suffix-autodl.json"))
		assert.Equal(t, "control_plane_singleton_pods", pods.TableName)
		require.Len(t, pods.Rows, 1)
		assert.Equal(t, map[string]string{
			"Namespace":          "openshift-a",
			"Deployment":         "a",
			"PodName":            "a-1",
			"NodeName":           "master-0",
			"MemoryRequestMB":    "50.00",
			"PodAgeSeconds":      "90",
			"StartMemoryUsageMB": "120.00",
			"EndMemoryUsageMB":   "250.00",
			"PeakMemoryUsageMB":  "300.00",
			"AvgMemoryUsageMB":   "200.00",
		}, pods.Rows[0])

		placement := readDataFile(t, filepath.Join(dir, "control-plane-singleton-placement_suffix-autodl.json"))
		assert.Equal(t, "control_plane_singleton_placement", placement.TableName)
		assert.Equal(t, []map[string]string{
			{"NodeName": "master-0", "SingletonPods": "1", "SingletonMemoryRequestMB": "50.00", "SingletonStartMemoryUsageMB": "120.00", "SingletonEndMemoryUsageMB": "250.00", "TotalMasters": "2"},
			{"NodeName": "master-1", "SingletonPods": "0", "SingletonMemoryRequestMB": "0.00", "TotalMasters": "2"},
		}, placement.Rows)
	})
}

func readDataFile(t *testing.T, path string) dataloader.DataFile {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var df dataloader.DataFile
	require.NoError(t, json.Unmarshal(raw, &df))
	return df
}
