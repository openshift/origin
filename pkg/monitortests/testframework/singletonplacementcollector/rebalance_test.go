package singletonplacementcollector

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodsToRebalance(t *testing.T) {
	masters := []string{"master-0", "master-1", "master-2"}
	pod := func(name, nodeName string, requestMB int64) singletonPod {
		return singletonPod{namespace: "openshift-a", podName: name, nodeName: nodeName, memoryRequestBytes: requestMB * bytesPerMB}
	}

	tests := []struct {
		name     string
		pods     []singletonPod
		expected []string
	}{
		{
			name: "balanced",
			pods: []singletonPod{pod("a", "master-0", 50), pod("b", "master-1", 50), pod("c", "master-2", 50), pod("d", "master-0", 50)},
		},
		{
			name: "one overloaded master sheds down to an even share, largest requests first",
			pods: []singletonPod{
				pod("a", "master-0", 10), pod("b", "master-0", 300), pod("c", "master-0", 50),
				pod("d", "master-0", 100), pod("e", "master-0", 50), pod("f", "master-1", 50),
			},
			expected: []string{"b", "d", "c"},
		},
		{
			name: "empty master after upgrade, both loaded masters shed",
			pods: []singletonPod{
				pod("a", "master-0", 50), pod("b", "master-0", 50), pod("c", "master-0", 50), pod("d", "master-0", 50), pod("e", "master-0", 50),
				pod("f", "master-1", 50), pod("g", "master-1", 50), pod("h", "master-1", 50), pod("i", "master-1", 50),
			},
			expected: []string{"a", "b", "f"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var names []string
			for _, p := range podsToRebalance(masters, tt.pods) {
				names = append(names, p.podName)
			}
			assert.Equal(t, tt.expected, names)
		})
	}
}

func readyMasters(notReady ...string) []runtime.Object {
	skip := map[string]bool{}
	for _, name := range notReady {
		skip[name] = true
	}
	objs := threeMasters()
	for _, obj := range objs {
		n := obj.(*corev1.Node)
		status := corev1.ConditionTrue
		if skip[n.Name] {
			status = corev1.ConditionFalse
		}
		n.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}}
	}
	return objs
}

// readySingleton is a singleton whose Deployment reports one ready replica, as it would once the
// replacement for a deleted pod is up.
func readySingleton(namespace, name, nodeName string) []runtime.Object {
	objs := singleton(namespace, name, nodeName, 1, "50Mi")
	objs[0].(*appsv1.Deployment).Status = appsv1.DeploymentStatus{Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	return objs
}

func remainingPods(t *testing.T, client *fake.Clientset) []string {
	pods, err := client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	var names []string
	for _, p := range pods.Items {
		names = append(names, p.Namespace)
	}
	sort.Strings(names)
	return names
}

func TestRebalanceSingletons(t *testing.T) {
	overloaded := func(nodes []runtime.Object) []runtime.Object {
		objs := nodes
		objs = append(objs, readySingleton("openshift-a", "a", "master-0")...)
		objs = append(objs, readySingleton("openshift-b", "b", "master-0")...)
		objs = append(objs, readySingleton("openshift-c", "c", "master-0")...)
		objs = append(objs, readySingleton("openshift-d", "d", "master-1")...)
		return objs
	}

	t.Run("deletes the excess from the overloaded master", func(t *testing.T) {
		client := fake.NewSimpleClientset(overloaded(readyMasters())...)
		require.NoError(t, rebalanceSingletons(context.Background(), client, 1, time.Millisecond, time.Second))
		// target is ceil(4/3) = 2, so one of master-0's three equal pods is deleted (first by name)
		assert.Equal(t, []string{"openshift-b", "openshift-c", "openshift-d"}, remainingPods(t, client))
	})

	t.Run("skips when a master is not ready", func(t *testing.T) {
		client := fake.NewSimpleClientset(overloaded(readyMasters("master-2"))...)
		require.NoError(t, rebalanceSingletons(context.Background(), client, 1, time.Millisecond, time.Second))
		assert.Equal(t, []string{"openshift-a", "openshift-b", "openshift-c", "openshift-d"}, remainingPods(t, client))
	})

	t.Run("errors when the replacement never becomes ready", func(t *testing.T) {
		objs := overloaded(readyMasters())
		objs[len(threeMasters())].(*appsv1.Deployment).Status.ReadyReplicas = 0
		client := fake.NewSimpleClientset(objs...)
		err := rebalanceSingletons(context.Background(), client, 1, time.Millisecond, 50*time.Millisecond)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "deployment openshift-a/a not ready")
	})
}
