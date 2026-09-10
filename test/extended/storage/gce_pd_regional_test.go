package storage

import (
	"context"
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestFamilyLacksPDStandard(t *testing.T) {
	tests := []struct {
		instanceType string
		want         bool
	}{
		// Bare families in the list.
		{"c3", true},
		{"c3d", true},
		{"c4", true},
		{"c4a", true},
		{"c4d", true},
		{"n4", true},
		// family+"-" variants.
		{"n4-standard-8", true},
		{"c3-highmem-4", true},
		{"c3d-standard-8", true},
		{"c4-standard-4", true},
		{"c4a-standard-16", true},
		{"c4d-standard-8", true},
		{"n4-custom-4-16384", true},
		// Prefix present but not followed by "-": must NOT match (delimiter guard).
		{"n4x-standard-8", false},
		{"c40-standard-4", false},
		// Substring but not prefix: HasPrefix, not Contains.
		{"custom-n4-foo", false},
		// Unrelated PD-capable families.
		{"n1-standard-8", false},
		{"n2-standard-4", false},
		{"e2-medium", false},
		// Case sensitivity: node labels are lowercase, contract is case-sensitive.
		{"N4-standard-8", false},
		// Empty.
		{"", false},
	}
	for _, tc := range tests {
		if got := familyLacksPDStandard(tc.instanceType); got != tc.want {
			t.Errorf("familyLacksPDStandard(%q) = %v, want %v", tc.instanceType, got, tc.want)
		}
	}
}

func TestNodeInstanceType(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name:   "stable label only",
			labels: map[string]string{v1.LabelInstanceTypeStable: "n4-standard-8"},
			want:   "n4-standard-8",
		},
		{
			name:   "beta label only",
			labels: map[string]string{v1.LabelInstanceType: "n4-standard-8"},
			want:   "n4-standard-8",
		},
		{
			name: "stable preferred over beta",
			labels: map[string]string{
				v1.LabelInstanceTypeStable: "c3-standard-4",
				v1.LabelInstanceType:       "n1-standard-4",
			},
			want: "c3-standard-4",
		},
		{
			name: "empty stable falls back to beta",
			labels: map[string]string{
				v1.LabelInstanceTypeStable: "",
				v1.LabelInstanceType:       "n1-standard-4",
			},
			want: "n1-standard-4",
		},
		{
			name:   "no instance-type label",
			labels: map[string]string{"foo": "bar"},
			want:   "",
		},
		{
			name:   "nil labels does not panic",
			labels: nil,
			want:   "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n", Labels: tc.labels}}
			if got := nodeInstanceType(node); got != tc.want {
				t.Errorf("nodeInstanceType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func roleNode(name, role, instanceType string) *v1.Node {
	labels := map[string]string{role: ""}
	if instanceType != "" {
		labels[v1.LabelInstanceTypeStable] = instanceType
	}
	return &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

func TestAnyNodeLacksPDStandard(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []runtime.Object
		selectors []string
		want      bool
	}{
		{
			name:      "no nodes",
			nodes:     nil,
			selectors: []string{labelNodeRoleWorker},
			want:      false,
		},
		{
			name:      "single hyperdisk-only worker",
			nodes:     []runtime.Object{roleNode("w0", labelNodeRoleWorker, "n4-standard-8")},
			selectors: []string{labelNodeRoleWorker},
			want:      true,
		},
		{
			name:      "single pd-capable worker",
			nodes:     []runtime.Object{roleNode("w0", labelNodeRoleWorker, "n1-standard-8")},
			selectors: []string{labelNodeRoleWorker},
			want:      false,
		},
		{
			name: "mixed workers - any hyperdisk-only",
			nodes: []runtime.Object{
				roleNode("w0", labelNodeRoleWorker, "n1-standard-8"),
				roleNode("w1", labelNodeRoleWorker, "n4-standard-8"),
			},
			selectors: []string{labelNodeRoleWorker},
			want:      true,
		},
		{
			name: "all pd-capable workers",
			nodes: []runtime.Object{
				roleNode("w0", labelNodeRoleWorker, "n1-standard-8"),
				roleNode("w1", labelNodeRoleWorker, "n2-standard-4"),
			},
			selectors: []string{labelNodeRoleWorker},
			want:      false,
		},
		{
			name: "worker with beta instance-type label",
			nodes: []runtime.Object{
				&v1.Node{ObjectMeta: metav1.ObjectMeta{
					Name:   "w0",
					Labels: map[string]string{labelNodeRoleWorker: "", v1.LabelInstanceType: "n4-standard-8"},
				}},
			},
			selectors: []string{labelNodeRoleWorker},
			want:      true,
		},
		{
			name:      "worker with no instance-type label",
			nodes:     []runtime.Object{roleNode("w0", labelNodeRoleWorker, "")},
			selectors: []string{labelNodeRoleWorker},
			want:      false,
		},
		{
			name:      "hyperdisk control-plane ignored under worker selector",
			nodes:     []runtime.Object{roleNode("m0", labelNodeRoleControlPlane, "n4-standard-8")},
			selectors: []string{labelNodeRoleWorker},
			want:      false,
		},
		{
			name:      "hyperdisk control-plane matched under control-plane selectors",
			nodes:     []runtime.Object{roleNode("m0", labelNodeRoleControlPlane, "n4-standard-8")},
			selectors: []string{labelNodeRoleControlPlane, labelNodeRoleMaster},
			want:      true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := fake.NewClientset(tc.nodes...)
			if got := anyNodeLacksPDStandard(context.Background(), client, tc.selectors...); got != tc.want {
				t.Errorf("anyNodeLacksPDStandard() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestAnyNodeLacksPDStandardListErrorFailsOpen locks in the fail-open behavior: a
// Nodes().List error is treated as "not found" so the test runs instead of skipping.
func TestAnyNodeLacksPDStandardListErrorFailsOpen(t *testing.T) {
	client := fake.NewClientset(roleNode("w0", labelNodeRoleWorker, "n4-standard-8"))
	client.PrependReactor("list", "nodes", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("injected list error")
	})
	if got := anyNodeLacksPDStandard(context.Background(), client, labelNodeRoleWorker); got != false {
		t.Errorf("anyNodeLacksPDStandard() with list error = %v, want false (fail open)", got)
	}
}
