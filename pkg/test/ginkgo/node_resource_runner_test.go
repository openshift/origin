package ginkgo

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseNodeResourceTag(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNum   int
		wantLabel string
		wantIsAll bool
		wantErr   bool
	}{
		{
			name:      "single node",
			input:     "[NodeResource:numNodes=1,label=foo]",
			wantNum:   1,
			wantLabel: "foo",
		},
		{
			name:      "all nodes",
			input:     "[NodeResource:numNodes=all,label=bar]",
			wantNum:   -1,
			wantLabel: "bar",
			wantIsAll: true,
		},
		{
			name:      "multiple nodes",
			input:     "[NodeResource:numNodes=3,label=multi_node]",
			wantNum:   3,
			wantLabel: "multi_node",
		},
		{
			name:      "embedded in full test name",
			input:     "[sig-node][Disruptive][NodeResource:numNodes=1,label=test_embed] some test description",
			wantNum:   1,
			wantLabel: "test_embed",
		},
		{
			name:    "zero nodes",
			input:   "[NodeResource:numNodes=0,label=bad]",
			wantErr: true,
		},
		{
			name:    "negative nodes",
			input:   "[NodeResource:numNodes=-1,label=bad]",
			wantErr: true,
		},
		{
			name:    "non-numeric nodes",
			input:   "[NodeResource:numNodes=abc,label=bad]",
			wantErr: true,
		},
		{
			name:    "no tag",
			input:   "no tag here",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseNodeResourceTag(tt.input)
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
		name string
		want bool
	}{
		{"[NodeResource:numNodes=1,label=x] test", true},
		{"[sig-node][NodeResource:numNodes=all,label=y] test", true},
		{"[sig-node] regular test", false},
		{"", false},
		{"NodeResource without brackets", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &testCase{name: tt.name}
			got := isNodeResourceTest(tc)
			if got != tt.want {
				t.Errorf("isNodeResourceTest(%q) = %v, want %v", tt.name, got, tt.want)
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
