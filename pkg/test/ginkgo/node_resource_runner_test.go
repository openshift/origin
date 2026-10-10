package ginkgo

import (
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/openshift-eng/openshift-tests-extension/pkg/util/sets"
	"github.com/openshift/origin/pkg/test/extensions"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeTestCaseWithLabels(name string, labels ...string) *testCase {
	return &testCase{
		name: name,
		spec: &extensions.ExtensionTestSpec{
			ExtensionTestSpec: &extensiontests.ExtensionTestSpec{
				Name:   name,
				Labels: sets.New[string](labels...),
			},
		},
	}
}

func TestParseNodeResourceFromSpec(t *testing.T) {
	tests := []struct {
		name      string
		labels    []string
		wantNum   int
		wantLabel string
		wantIsAll bool
		wantErr   bool
	}{
		{
			name:      "single node",
			labels:    []string{"NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=foo"},
			wantNum:   1,
			wantLabel: "foo",
		},
		{
			name:      "all nodes",
			labels:    []string{"NodeResource", "NodeResourceNumNodes=all", "NodeResourceName=bar"},
			wantNum:   -1,
			wantLabel: "bar",
			wantIsAll: true,
		},
		{
			name:      "multiple nodes",
			labels:    []string{"NodeResource", "NodeResourceNumNodes=3", "NodeResourceName=multi_node"},
			wantNum:   3,
			wantLabel: "multi_node",
		},
		{
			name:      "defaults to 1 node when numNodes not specified",
			labels:    []string{"NodeResource", "NodeResourceName=default_test"},
			wantNum:   1,
			wantLabel: "default_test",
		},
		{
			name:      "max wins with multiple numNodes labels",
			labels:    []string{"NodeResource", "NodeResourceNumNodes=1", "NodeResourceNumNodes=2", "NodeResourceName=override"},
			wantNum:   2,
			wantLabel: "override",
		},
		{
			name:    "missing NodeResource label",
			labels:  []string{"NodeResourceNumNodes=1", "NodeResourceName=foo"},
			wantErr: true,
		},
		{
			name:    "missing NodeResourceName",
			labels:  []string{"NodeResource", "NodeResourceNumNodes=1"},
			wantErr: true,
		},
		{
			name:    "non-numeric nodes",
			labels:  []string{"NodeResource", "NodeResourceNumNodes=abc", "NodeResourceName=bad"},
			wantErr: true,
		},
		{
			name:    "nil spec",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tc *testCase
			if tt.labels == nil {
				tc = &testCase{name: "no-spec"}
			} else {
				tc = makeTestCaseWithLabels("test", tt.labels...)
			}

			cfg, err := parseNodeResourceFromSpec(tc)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.numNodes != tt.wantNum {
				t.Errorf("numNodes = %d, want %d", cfg.numNodes, tt.wantNum)
			}
			if cfg.label != tt.wantLabel {
				t.Errorf("label = %q, want %q", cfg.label, tt.wantLabel)
			}
			if cfg.isAll != tt.wantIsAll {
				t.Errorf("isAll = %v, want %v", cfg.isAll, tt.wantIsAll)
			}
		})
	}
}

func TestIsNodeResourceTest(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		want   bool
	}{
		{"with NodeResource label", []string{"NodeResource", "NodeResourceNumNodes=1", "NodeResourceName=x"}, true},
		{"without NodeResource label", []string{"sig-node"}, false},
		{"nil spec", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tc *testCase
			if tt.labels == nil {
				tc = &testCase{name: "no-spec"}
			} else {
				tc = makeTestCaseWithLabels("test", tt.labels...)
			}
			got := isNodeResourceTest(tc)
			if got != tt.want {
				t.Errorf("isNodeResourceTest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		name       string
		conditions []corev1.NodeCondition
		want       bool
	}{
		{
			name: "ready true",
			conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			want: true,
		},
		{
			name: "ready false",
			conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
			},
			want: false,
		},
		{
			name: "ready unknown",
			conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionUnknown},
			},
			want: false,
		},
		{
			name:       "no conditions",
			conditions: nil,
			want:       false,
		},
		{
			name: "other conditions only",
			conditions: []corev1.NodeCondition{
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
			},
			want: false,
		},
		{
			name: "ready true among other conditions",
			conditions: []corev1.NodeCondition{
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
				Status:     corev1.NodeStatus{Conditions: tt.conditions},
			}
			got := isNodeReady(node)
			if got != tt.want {
				t.Errorf("isNodeReady() = %v, want %v", got, tt.want)
			}
		})
	}
}
