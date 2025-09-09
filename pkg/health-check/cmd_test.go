package health_check

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

func TestGetUnreadyOrUnschedulableNodeNames(t *testing.T) {
	tests := []struct {
		name     string
		nodes    *k8sv1.NodeList
		expected []string
	}{
		{
			name: "all nodes ready",
			nodes: &k8sv1.NodeList{
				Items: []k8sv1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
				},
			},
			expected: []string{},
		},
		{
			name: "one node not ready",
			nodes: &k8sv1.NodeList{
				Items: []k8sv1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionFalse}}}},
				},
			},
			expected: []string{"node-2"},
		},
		{
			name: "one node unschedulable",
			nodes: &k8sv1.NodeList{
				Items: []k8sv1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}, Spec: k8sv1.NodeSpec{Unschedulable: true}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
				},
			},
			expected: []string{"node-2"},
		},
		{
			name: "mixed not ready and unschedulable",
			nodes: &k8sv1.NodeList{
				Items: []k8sv1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionFalse}}}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-3"}, Spec: k8sv1.NodeSpec{Unschedulable: true}, Status: k8sv1.NodeStatus{Conditions: []k8sv1.NodeCondition{{Type: k8sv1.NodeReady, Status: k8sv1.ConditionTrue}}}},
				},
			},
			expected: []string{"node-2", "node-3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := getUnreadyOrUnschedulableNodeNames(tc.nodes)
			assert.ElementsMatch(t, tc.expected, actual)
		})
	}
}
