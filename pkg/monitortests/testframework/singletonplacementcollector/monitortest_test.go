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

// ownedPod creates a Running pod on nodeName controlled by an owner of the given kind, or with no
// controller when ownerKind is empty.
func ownedPod(namespace, name, nodeName, ownerKind, ownerName string) []runtime.Object {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         namespace,
			Name:              name,
			CreationTimestamp: metav1.NewTime(testNow.Add(-time.Hour)),
		},
		Spec:   corev1.PodSpec{NodeName: nodeName},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	if ownerKind != "" {
		pod.OwnerReferences = []metav1.OwnerReference{{Kind: ownerKind, Name: ownerName, Controller: ptr.To(true)}}
	}
	return []runtime.Object{pod}
}

// multiReplica creates a Deployment and ReplicaSet with one Running pod per node.
func multiReplica(namespace, name string, nodeNames ...string) []runtime.Object {
	objs := singleton(namespace, name, nodeNames[0], int32(len(nodeNames)))
	rsName := name + "-abc123"
	for _, nodeName := range nodeNames[1:] {
		objs = append(objs, ownedPod(namespace, rsName+"-"+nodeName, nodeName, "ReplicaSet", rsName)...)
	}
	return objs
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
		// expected pod count per master; nil means nothing should be recorded
		expectedPerMaster map[string]int
		// expected singleton count per master; defaults to expectedPerMaster
		expectedSingletonsPerMaster map[string]int
		expectedPods                []string
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
			name: "multi-replica deployments included",
			objs: objects(threeMasters(),
				singleton("openshift-a", "a", "master-0", 1, "50Mi"),
				multiReplica("openshift-b", "b", "master-0", "master-1"),
			),
			expectedPerMaster:           map[string]int{"master-0": 2, "master-1": 1, "master-2": 0},
			expectedSingletonsPerMaster: map[string]int{"master-0": 1, "master-1": 0, "master-2": 0},
			expectedPods:                []string{"a-abc123-xyz", "b-abc123-master-1", "b-abc123-xyz"},
		},
		{
			name: "daemonset, static, guard and catalog pods included",
			objs: objects(threeMasters(),
				ownedPod("openshift-dns", "dns-default-x1", "master-0", "DaemonSet", "dns-default"),
				ownedPod("openshift-etcd", "etcd-master-1", "master-1", "Node", "master-1"),
				ownedPod("openshift-etcd", "etcd-guard-master-1", "master-1", "", ""),
				ownedPod("openshift-marketplace", "redhat-operators-x1", "master-2", "CatalogSource", "redhat-operators"),
			),
			expectedPerMaster:           map[string]int{"master-0": 1, "master-1": 2, "master-2": 1},
			expectedSingletonsPerMaster: map[string]int{"master-0": 0, "master-1": 0, "master-2": 0},
			expectedPods:                []string{"dns-default-x1", "etcd-guard-master-1", "etcd-master-1", "redhat-operators-x1"},
		},
		{
			name: "non-openshift namespaces excluded",
			objs: objects(threeMasters(),
				singleton("openshift-a", "a", "master-0", 1, "50Mi"),
				singleton("kube-system", "b", "master-0", 1, "50Mi"),
				singleton("e2e-test-foo", "c", "master-1", 1, "50Mi"),
				ownedPod("e2e-test-bar", "d", "master-1", "", ""),
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
			name: "single node is collected",
			objs: objects(
				[]runtime.Object{node("master-0", "master"), node("worker-0", "worker")},
				singleton("openshift-a", "a", "master-0", 1, "50Mi"),
			),
			expectedPerMaster: map[string]int{"master-0": 1},
			expectedPods:      []string{"a-abc123-xyz"},
		},
		{
			name: "two node fencing is collected",
			objs: objects(
				[]runtime.Object{node("master-0", "master"), node("master-1", "master")},
				singleton("openshift-a", "a", "master-0", 1, "50Mi"),
				singleton("openshift-b", "b", "master-1", 1, "50Mi"),
			),
			expectedPerMaster: map[string]int{"master-0": 1, "master-1": 1},
			expectedPods:      []string{"a-abc123-xyz", "b-abc123-xyz"},
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
			actualSingletonsPerMaster := map[string]int{}
			for _, p := range w.placement {
				actualPerMaster[p.nodeName] = p.all.pods
				actualSingletonsPerMaster[p.nodeName] = p.singletons.pods
			}
			assert.Equal(t, tc.expectedPerMaster, actualPerMaster)
			expectedSingletons := tc.expectedSingletonsPerMaster
			if expectedSingletons == nil {
				expectedSingletons = tc.expectedPerMaster
			}
			assert.Equal(t, expectedSingletons, actualSingletonsPerMaster)

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
			assert.Equal(t, "a", w.pods[0].workload)
			assert.Equal(t, "Deployment", w.pods[0].workloadKind)
			assert.Equal(t, ptr.To(int32(1)), w.pods[0].replicas)
			assert.Equal(t, int64(80*bytesPerMB), w.pods[0].memoryRequestBytes)
			assert.Equal(t, time.Hour, w.pods[0].podAge)
			assert.Equal(t, tc.expectedPodBPeak, w.pods[1].peakUsageBytes)

			require.Len(t, w.placement, 3)
			assert.Equal(t, "master-0", w.placement[0].nodeName)
			// both pods are singletons, so the totals match
			for _, totals := range []func(masterPlacement) podTotals{
				func(p masterPlacement) podTotals { return p.all },
				func(p masterPlacement) podTotals { return p.singletons },
			} {
				master0, master1 := totals(w.placement[0]), totals(w.placement[1])
				assert.Equal(t, 2, master0.pods)
				assert.Equal(t, int64(180*bytesPerMB), master0.memoryRequestBytes)
				assert.Equal(t, tc.expectedMaster0Current, master0.currentUsageBytes)
				assert.Equal(t, tc.expectedMaster0Peak, master0.peakUsageBytes)
				assert.Equal(t, tc.expectedMaster0Avg, master0.avgUsageBytes)
				assert.Equal(t, tc.expectedMaster1Current, master1.currentUsageBytes)
			}
		})
	}
}

func TestCollectStartUsage(t *testing.T) {
	mb := func(v float64) float64 { return v * bytesPerMB }
	startObjs := objects(threeMasters(),
		singleton("openshift-a", "a", "master-0", 1, "50Mi"),
		singleton("openshift-b", "b", "master-0", 1, "50Mi"),
	)
	startObjs = append(startObjs, multiReplica("openshift-m", "m", "master-0", "master-1")...)
	endObjs := objects(threeMasters(),
		singleton("openshift-a", "a", "master-0", 1, "50Mi"),
		replaced(singleton("openshift-b", "b", "master-0", 1, "50Mi"), "b-abc123-new", "master-1"),
		singleton("openshift-c", "c", "master-1", 1, "50Mi"),
		// m-abc123-xyz survives, the master-1 replica is replaced
		singleton("openshift-m", "m", "master-0", 2, "50Mi"),
		ownedPod("openshift-m", "m-abc123-new", "master-2", "ReplicaSet", "m-abc123"),
	)

	testCases := []struct {
		name       string
		startUsage map[string]float64

		// expected start usage per end pod name, and per master in master order
		expectedPods      map[string]*float64
		expectedPlacement []*float64
	}{
		{
			name: "start usage joined by pod, falling back to workload mean",
			startUsage: map[string]float64{
				"openshift-a/a-abc123-xyz":      mb(100),
				"openshift-b/b-abc123-xyz":      mb(40),
				"openshift-m/m-abc123-xyz":      mb(10),
				"openshift-m/m-abc123-master-1": mb(30),
			},
			expectedPods: map[string]*float64{
				"a-abc123-xyz": ptr.To(mb(100)),
				// replaced singleton keeps its deployment's start value
				"b-abc123-new": ptr.To(mb(40)),
				// deployment created during the run has no start value
				"c-abc123-xyz": nil,
				// surviving replica keeps its own start value
				"m-abc123-xyz": ptr.To(mb(10)),
				// replaced replica gets the mean of the deployment's replicas at start
				"m-abc123-new": ptr.To(mb(20)),
			},
			expectedPlacement: []*float64{ptr.To(mb(110)), ptr.To(mb(40)), ptr.To(mb(20))},
		},
		{
			name: "workload mean only averages replicas with a start sample",
			startUsage: map[string]float64{
				"openshift-a/a-abc123-xyz": mb(100),
				"openshift-b/b-abc123-xyz": mb(40),
				"openshift-m/m-abc123-xyz": mb(10),
			},
			expectedPods: map[string]*float64{
				"a-abc123-xyz": ptr.To(mb(100)),
				"b-abc123-new": ptr.To(mb(40)),
				"c-abc123-xyz": nil,
				"m-abc123-xyz": ptr.To(mb(10)),
				// the master-1 replica had no sample, so it does not drag the mean down to 5
				"m-abc123-new": ptr.To(mb(10)),
			},
			expectedPlacement: []*float64{ptr.To(mb(110)), ptr.To(mb(40)), ptr.To(mb(10))},
		},
		{
			name: "metrics API unavailable at start leaves start usage unset",
			expectedPods: map[string]*float64{
				"a-abc123-xyz": nil,
				"b-abc123-new": nil,
				"c-abc123-xyz": nil,
				"m-abc123-xyz": nil,
				"m-abc123-new": nil,
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
				actualPlacement = append(actualPlacement, p.all.startUsageBytes)
			}
			assert.Equal(t, tc.expectedPlacement, actualPlacement)
		})
	}
}

func TestListPodWorkloads(t *testing.T) {
	objs := objects(threeMasters(),
		singleton("openshift-a", "a", "master-0", 1, "50Mi"),
		multiReplica("openshift-b", "b", "master-0", "master-1"),
		ownedPod("openshift-dns", "dns-default-x1", "master-0", "DaemonSet", "dns-default"),
		ownedPod("openshift-etcd", "etcd-master-1", "master-1", "Node", "master-1"),
		ownedPod("openshift-etcd", "etcd-guard-master-1", "master-1", "", ""),
		ownedPod("openshift-r", "r-1", "master-2", "ReplicaSet", "bare-rs"),
	)
	client := fake.NewSimpleClientset(objs...)

	type workload struct {
		kind, name string
		replicas   *int32
	}
	toMap := func(pods []controlPlanePod) map[string]workload {
		ret := map[string]workload{}
		for _, p := range pods {
			ret[p.podName] = workload{p.workloadKind, p.workload, p.replicas}
		}
		return ret
	}

	_, pods, err := listControlPlanePods(context.Background(), client, testNow)
	require.NoError(t, err)
	assert.Equal(t, map[string]workload{
		"a-abc123-xyz":        {"Deployment", "a", ptr.To(int32(1))},
		"b-abc123-xyz":        {"Deployment", "b", ptr.To(int32(2))},
		"b-abc123-master-1":   {"Deployment", "b", ptr.To(int32(2))},
		"dns-default-x1":      {"DaemonSet", "dns-default", nil},
		"etcd-master-1":       {"StaticPod", "etcd", nil},
		"etcd-guard-master-1": {"Pod", "etcd-guard", nil},
		"r-1":                 {"ReplicaSet", "bare-rs", nil},
	}, toMap(pods))

	// RebalanceSingletons must only ever see single-replica Deployments.
	_, singletons, err := listSingletonPods(context.Background(), client, testNow)
	require.NoError(t, err)
	assert.Equal(t, map[string]workload{
		"a-abc123-xyz": {"Deployment", "a", ptr.To(int32(1))},
	}, toMap(singletons))
}

func TestSummarizePlacementUsage(t *testing.T) {
	masters := []string{"master-0", "master-1", "master-2"}

	testCases := []struct {
		name             string
		pods             []controlPlanePod
		haveCurrentUsage bool
		// expected currentUsageBytes per master, in master order
		expected []*float64
	}{
		{
			name: "partial samples on a master are summed",
			pods: []controlPlanePod{
				{nodeName: "master-0", currentUsageBytes: ptr.To(100.0)},
				{nodeName: "master-0"},
				{nodeName: "master-1", currentUsageBytes: ptr.To(50.0)},
			},
			haveCurrentUsage: true,
			expected:         []*float64{ptr.To(100.0), ptr.To(50.0), ptr.To(0.0)},
		},
		{
			name: "master whose singletons all lack samples is left unset",
			pods: []controlPlanePod{
				{nodeName: "master-0", currentUsageBytes: ptr.To(100.0)},
				{nodeName: "master-1"},
				{nodeName: "master-1"},
			},
			haveCurrentUsage: true,
			expected:         []*float64{ptr.To(100.0), nil, ptr.To(0.0)},
		},
		{
			name: "source without data leaves every master unset",
			pods: []controlPlanePod{
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
				actual = append(actual, p.all.currentUsageBytes)
				assert.Nil(t, p.all.startUsageBytes)
				assert.Nil(t, p.all.peakUsageBytes)
				assert.Nil(t, p.all.avgUsageBytes)
			}
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestSummarizePlacementSingletons(t *testing.T) {
	singletonPod := func(nodeName string, requestMB int64, usage float64) controlPlanePod {
		return controlPlanePod{workloadKind: "Deployment", replicas: ptr.To(int32(1)), nodeName: nodeName, memoryRequestBytes: requestMB * bytesPerMB, currentUsageBytes: ptr.To(usage)}
	}
	pods := []controlPlanePod{
		singletonPod("master-0", 50, 100),
		{workloadKind: "Deployment", replicas: ptr.To(int32(3)), nodeName: "master-0", memoryRequestBytes: 200 * bytesPerMB, currentUsageBytes: ptr.To(400.0)},
		{workloadKind: "DaemonSet", nodeName: "master-1", memoryRequestBytes: 10 * bytesPerMB, currentUsageBytes: ptr.To(30.0)},
	}

	placement := summarizePlacement([]string{"master-0", "master-1", "master-2"}, pods, false, true, false)
	require.Len(t, placement, 3)

	assert.Equal(t, podTotals{pods: 2, memoryRequestBytes: 250 * bytesPerMB, currentUsageBytes: ptr.To(500.0)}, placement[0].all)
	assert.Equal(t, podTotals{pods: 1, memoryRequestBytes: 50 * bytesPerMB, currentUsageBytes: ptr.To(100.0)}, placement[0].singletons)
	assert.Equal(t, podTotals{pods: 1, memoryRequestBytes: 10 * bytesPerMB, currentUsageBytes: ptr.To(30.0)}, placement[1].all)
	// a master with pods but no singletons still gets a zero singleton usage sum
	assert.Equal(t, podTotals{currentUsageBytes: ptr.To(0.0)}, placement[1].singletons)
	assert.Equal(t, podTotals{currentUsageBytes: ptr.To(0.0)}, placement[2].all)
	assert.Equal(t, podTotals{currentUsageBytes: ptr.To(0.0)}, placement[2].singletons)
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
			pods: []controlPlanePod{{
				namespace: "openshift-a", workloadKind: "Deployment", workload: "a", replicas: ptr.To(int32(1)), podName: "a-1", nodeName: "master-0",
				memoryRequestBytes: 50 * bytesPerMB, podAge: 90 * time.Second,
				startUsageBytes: ptr.To(120.0 * bytesPerMB), currentUsageBytes: ptr.To(250.0 * bytesPerMB),
				peakUsageBytes: ptr.To(300.0 * bytesPerMB), avgUsageBytes: ptr.To(200.0 * bytesPerMB),
			}},
			placement: []masterPlacement{
				{
					nodeName:   "master-0",
					singletons: podTotals{pods: 1, memoryRequestBytes: 50 * bytesPerMB, startUsageBytes: ptr.To(120.0 * bytesPerMB), currentUsageBytes: ptr.To(250.0 * bytesPerMB)},
					all: podTotals{pods: 3, memoryRequestBytes: 150 * bytesPerMB, startUsageBytes: ptr.To(320.0 * bytesPerMB), currentUsageBytes: ptr.To(650.0 * bytesPerMB),
						peakUsageBytes: ptr.To(700.0 * bytesPerMB), avgUsageBytes: ptr.To(500.0 * bytesPerMB)},
				},
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
			"WorkloadKind":       "Deployment",
			"Replicas":           "1",
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
		assert.Len(t, placement.Schema, 14)
		assert.Equal(t, []map[string]string{
			{
				"NodeName": "master-0", "TotalMasters": "2",
				"SingletonPods": "1", "SingletonMemoryRequestMB": "50.00", "SingletonStartMemoryUsageMB": "120.00", "SingletonEndMemoryUsageMB": "250.00",
				"AllPods": "3", "AllMemoryRequestMB": "150.00", "AllStartMemoryUsageMB": "320.00", "AllEndMemoryUsageMB": "650.00",
				"AllPeakMemoryUsageMB": "700.00", "AllAvgMemoryUsageMB": "500.00",
			},
			{
				"NodeName": "master-1", "TotalMasters": "2",
				"SingletonPods": "0", "SingletonMemoryRequestMB": "0.00",
				"AllPods": "0", "AllMemoryRequestMB": "0.00",
			},
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
