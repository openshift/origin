package e2e_analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExpandDependencies(t *testing.T) {
	tests := []struct {
		name     string
		deps     map[string][]string
		expected map[string][]string
	}{
		{
			name: "simple case",
			deps: map[string][]string{
				"A": {"B"},
				"B": {"C"},
			},
			expected: map[string][]string{
				"A": {"B", "C"},
				"B": {"C"},
				"C": {},
			},
		},
		{
			name: "multiple dependencies",
			deps: map[string][]string{
				"A": {"B", "C"},
				"B": {"D"},
				"C": {"D"},
			},
			expected: map[string][]string{
				"A": {"B", "C", "D"},
				"B": {"D"},
				"C": {"D"},
				"D": {},
			},
		},
		{
			name: "no dependencies",
			deps: map[string][]string{
				"A": {},
				"B": {},
			},
			expected: map[string][]string{
				"A": {},
				"B": {},
			},
		},
		{
			name: "complex chain",
			deps: map[string][]string{
				"etcd":                         {},
				"network":                      {"etcd"},
				"kube-apiserver":               {"etcd", "network"},
				"kube-controller-manager":      {"kube-apiserver"},
				"openshift-controller-manager": {"openshift-apiserver", "kube-apiserver"},
				"openshift-apiserver":          {"kube-apiserver"},
			},
			expected: map[string][]string{
				"etcd":                         {},
				"network":                      {"etcd"},
				"kube-apiserver":               {"etcd", "network"},
				"kube-controller-manager":      {"etcd", "kube-apiserver", "network"},
				"openshift-apiserver":          {"etcd", "kube-apiserver", "network"},
				"openshift-controller-manager": {"etcd", "kube-apiserver", "network", "openshift-apiserver"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := expandDependencies(tc.deps)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestTopologicalSort(t *testing.T) {
	tests := []struct {
		name         string
		operators    []string
		dependencies map[string][]string
		expected     []string
		expectError  bool
	}{
		{
			name:      "linear dependency",
			operators: []string{"A", "B", "C"},
			dependencies: map[string][]string{
				"A": {},
				"B": {"A"},
				"C": {"B"},
			},
			expected:    []string{"A", "B", "C"},
			expectError: false,
		},
		{
			name:      "multiple dependencies",
			operators: []string{"A", "B", "C", "D"},
			dependencies: map[string][]string{
				"A": {},
				"B": {"A"},
				"C": {"A"},
				"D": {"B", "C"},
			},
			expected:    []string{"A", "B", "C", "D"}, // Order of B and C can vary
			expectError: false,
		},
		{
			name:      "cycle detection",
			operators: []string{"A", "B", "C"},
			dependencies: map[string][]string{
				"A": {"C"},
				"B": {"A"},
				"C": {"B"},
			},
			expected:    nil,
			expectError: true,
		},
		{
			name:      "disconnected components",
			operators: []string{"A", "B", "C", "D"},
			dependencies: map[string][]string{
				"A": {},
				"B": {"A"},
				"C": {},
				"D": {"C"},
			},
			expected:    []string{"A", "C", "B", "D"}, // Order can vary
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sorted, err := TopologicalSort(tc.operators, tc.dependencies)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Sorting the results because the order of elements with the same in-degree can vary
				assert.ElementsMatch(t, tc.expected, sorted)
			}
		})
	}
}

func TestGetUnreadyOrUnschedulableNodes(t *testing.T) {
	tests := []struct {
		name     string
		nodes    *k8sv1.NodeList
		expected []unreadyNode
	}{
		{
			name: "all nodes ready",
			nodes: &k8sv1.NodeList{
				Items: []k8sv1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
				},
			},
			expected: nil,
		},
		{
			name: "one node not ready includes problematic conditions",
			nodes: &k8sv1.NodeList{
				Items: []k8sv1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{
						{Type: k8sv1.NodeReady, Status: k8sv1.ConditionFalse, Reason: "KubeletNotReady", Message: "container runtime not ready"},
						{Type: k8sv1.NodeMemoryPressure, Status: k8sv1.ConditionFalse},
					}}},
				},
			},
			expected: []unreadyNode{
				{
					name: "node-2",
					conditions: []k8sv1.NodeCondition{
						{Type: k8sv1.NodeReady, Status: k8sv1.ConditionFalse, Reason: "KubeletNotReady", Message: "container runtime not ready"},
					},
				},
			},
		},
		{
			name: "unschedulable node with no problematic conditions sets flag",
			nodes: &k8sv1.NodeList{
				Items: []k8sv1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}, Spec: k8sv1.NodeSpec{Unschedulable: true}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
				},
			},
			expected: []unreadyNode{
				{name: "node-2", unschedulable: true},
			},
		},
		{
			name: "node with all conditions unknown",
			nodes: &k8sv1.NodeList{
				Items: []k8sv1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{
						{Type: k8sv1.NodeReady, Status: k8sv1.ConditionUnknown, Reason: "NodeStatusUnknown", Message: "Kubelet stopped posting node status."},
						{Type: k8sv1.NodeMemoryPressure, Status: k8sv1.ConditionUnknown, Reason: "NodeStatusUnknown"},
						{Type: k8sv1.NodeDiskPressure, Status: k8sv1.ConditionUnknown, Reason: "NodeStatusUnknown"},
						{Type: k8sv1.NodePIDPressure, Status: k8sv1.ConditionUnknown, Reason: "NodeStatusUnknown"},
					}}},
				},
			},
			expected: []unreadyNode{
				{
					name: "node-1",
					conditions: []k8sv1.NodeCondition{
						{Type: k8sv1.NodeReady, Status: k8sv1.ConditionUnknown, Reason: "NodeStatusUnknown", Message: "Kubelet stopped posting node status."},
						{Type: k8sv1.NodeMemoryPressure, Status: k8sv1.ConditionUnknown, Reason: "NodeStatusUnknown"},
						{Type: k8sv1.NodeDiskPressure, Status: k8sv1.ConditionUnknown, Reason: "NodeStatusUnknown"},
						{Type: k8sv1.NodePIDPressure, Status: k8sv1.ConditionUnknown, Reason: "NodeStatusUnknown"},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := getUnreadyOrUnschedulableNodes(tc.nodes)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestIsProblematicCondition(t *testing.T) {
	tests := []struct {
		name     string
		cond     k8sv1.NodeCondition
		expected bool
	}{
		{
			name:     "Ready=True is not problematic",
			cond:     k8sv1.NodeCondition{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue},
			expected: false,
		},
		{
			name:     "Ready=False is problematic",
			cond:     k8sv1.NodeCondition{Type: k8sv1.NodeReady, Status: k8sv1.ConditionFalse},
			expected: true,
		},
		{
			name:     "Ready=Unknown is problematic",
			cond:     k8sv1.NodeCondition{Type: k8sv1.NodeReady, Status: k8sv1.ConditionUnknown},
			expected: true,
		},
		{
			name:     "MemoryPressure=False is not problematic",
			cond:     k8sv1.NodeCondition{Type: k8sv1.NodeMemoryPressure, Status: k8sv1.ConditionFalse},
			expected: false,
		},
		{
			name:     "MemoryPressure=True is problematic",
			cond:     k8sv1.NodeCondition{Type: k8sv1.NodeMemoryPressure, Status: k8sv1.ConditionTrue},
			expected: true,
		},
		{
			name:     "DiskPressure=Unknown is problematic",
			cond:     k8sv1.NodeCondition{Type: k8sv1.NodeDiskPressure, Status: k8sv1.ConditionUnknown},
			expected: true,
		},
		{
			name:     "PIDPressure=False is not problematic",
			cond:     k8sv1.NodeCondition{Type: k8sv1.NodePIDPressure, Status: k8sv1.ConditionFalse},
			expected: false,
		},
		{
			name:     "PIDPressure=True is problematic",
			cond:     k8sv1.NodeCondition{Type: k8sv1.NodePIDPressure, Status: k8sv1.ConditionTrue},
			expected: true,
		},
		{
			name:     "NetworkUnavailable=False is not problematic",
			cond:     k8sv1.NodeCondition{Type: k8sv1.NodeNetworkUnavailable, Status: k8sv1.ConditionFalse},
			expected: false,
		},
		{
			name:     "NetworkUnavailable=True is problematic",
			cond:     k8sv1.NodeCondition{Type: k8sv1.NodeNetworkUnavailable, Status: k8sv1.ConditionTrue},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isProblematicCondition(tc.cond))
		})
	}
}
