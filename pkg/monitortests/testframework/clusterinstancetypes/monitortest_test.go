package clusterinstancetypes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeNode(name, role, instanceType, region, zone string) corev1.Node {
	labels := map[string]string{
		"node.kubernetes.io/instance-type": instanceType,
		"topology.kubernetes.io/region":    region,
		"topology.kubernetes.io/zone":      zone,
	}
	if role == "master" {
		labels["node-role.kubernetes.io/master"] = ""
		labels["node-role.kubernetes.io/control-plane"] = ""
	} else {
		labels["node-role.kubernetes.io/worker"] = ""
	}
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func TestBuildRowsDeduplicates(t *testing.T) {
	nodes := []corev1.Node{
		makeNode("master-0", "master", "m6i.xlarge", "us-east-1", "us-east-1a"),
		makeNode("master-1", "master", "m6i.xlarge", "us-east-1", "us-east-1b"),
		makeNode("master-2", "master", "m6i.xlarge", "us-east-1", "us-east-1c"),
		makeNode("worker-0", "worker", "m6i.2xlarge", "us-east-1", "us-east-1a"),
		makeNode("worker-1", "worker", "m6i.2xlarge", "us-east-1", "us-east-1b"),
		makeNode("worker-2", "worker", "m6i.2xlarge", "us-east-1", "us-east-1a"),
	}

	rows := buildRows("aws", nodes)

	// 3 CP zones + 2 distinct worker zones = 5 rows
	if len(rows) != 5 {
		t.Fatalf("expected 5 deduplicated rows, got %d: %+v", len(rows), rows)
	}
	for _, r := range rows {
		if r.Zone == "" {
			t.Errorf("expected zone to be set: %+v", r)
		}
	}
}

func TestBuildRowsMixedWorkerTypes(t *testing.T) {
	nodes := []corev1.Node{
		makeNode("master-0", "master", "m6i.xlarge", "us-east-1", "us-east-1a"),
		makeNode("worker-0", "worker", "m5.xlarge", "us-east-1", "us-east-1a"),
		makeNode("worker-1", "worker", "m6i.2xlarge", "us-east-1", "us-east-1a"),
		makeNode("worker-2", "worker", "m5.xlarge", "us-east-1", "us-east-1a"),
	}

	rows := buildRows("aws", nodes)

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (1 cp + 2 distinct worker types), got %d: %+v", len(rows), rows)
	}
	if rows[0].Role != "control-plane" {
		t.Errorf("first row should be control-plane, got %+v", rows[0])
	}
	workerTypes := map[string]bool{}
	for _, r := range rows[1:] {
		if r.Role != "worker" {
			t.Errorf("expected worker role, got %+v", r)
		}
		workerTypes[r.InstanceType] = true
	}
	if !workerTypes["m5.xlarge"] || !workerTypes["m6i.2xlarge"] {
		t.Errorf("expected both worker types present, got %v", workerTypes)
	}
}

func TestBuildRowsSortsControlPlaneFirst(t *testing.T) {
	nodes := []corev1.Node{
		makeNode("worker-0", "worker", "m5.xlarge", "us-east-1", "us-east-1a"),
		makeNode("master-0", "master", "m6i.xlarge", "us-east-1", "us-east-1a"),
	}

	rows := buildRows("aws", nodes)

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Role != "control-plane" {
		t.Errorf("control-plane should sort first, got %+v", rows[0])
	}
}

func TestBuildRowsPropagatesPlatformRegionAndZone(t *testing.T) {
	nodes := []corev1.Node{
		makeNode("master-0", "master", "m6i.xlarge", "eu-west-1", "eu-west-1a"),
	}

	rows := buildRows("aws", nodes)

	if rows[0].Platform != "aws" || rows[0].Region != "eu-west-1" || rows[0].Zone != "eu-west-1a" {
		t.Errorf("expected platform=aws region=eu-west-1 zone=eu-west-1a, got %+v", rows[0])
	}
}

func TestBuildRowsWavelengthZones(t *testing.T) {
	nodes := []corev1.Node{
		makeNode("master-0", "master", "m6i.xlarge", "us-east-1", "us-east-1a"),
		makeNode("worker-0", "worker", "m6i.2xlarge", "us-east-1", "us-east-1a"),
		makeNode("worker-edge-0", "worker", "r5.2xlarge", "us-east-1", "us-east-1-wl1-bos-wlz-1"),
	}

	rows := buildRows("aws", nodes)

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (1 cp + 1 regular worker + 1 wavelength worker), got %d: %+v", len(rows), rows)
	}
	foundWavelength := false
	for _, r := range rows {
		if r.Zone == "us-east-1-wl1-bos-wlz-1" {
			foundWavelength = true
			if r.InstanceType != "r5.2xlarge" {
				t.Errorf("expected wavelength node instance type r5.2xlarge, got %s", r.InstanceType)
			}
		}
	}
	if !foundWavelength {
		t.Errorf("expected wavelength zone row, got %+v", rows)
	}
}

func TestBuildRowsSkipsNodesWithoutInstanceType(t *testing.T) {
	nodes := []corev1.Node{
		makeNode("master-0", "master", "m6i.xlarge", "us-east-1", "us-east-1a"),
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "worker-no-labels",
				Labels: map[string]string{
					"node-role.kubernetes.io/worker": "",
				},
			},
		},
	}

	rows := buildRows("aws", nodes)

	if len(rows) != 1 {
		t.Fatalf("expected 1 row (node without instance-type skipped), got %d: %+v", len(rows), rows)
	}
}

func TestNodeRole(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name:     "master label",
			labels:   map[string]string{"node-role.kubernetes.io/master": ""},
			expected: "control-plane",
		},
		{
			name:     "control-plane label",
			labels:   map[string]string{"node-role.kubernetes.io/control-plane": ""},
			expected: "control-plane",
		},
		{
			name:     "both labels",
			labels:   map[string]string{"node-role.kubernetes.io/master": "", "node-role.kubernetes.io/control-plane": ""},
			expected: "control-plane",
		},
		{
			name:     "worker label",
			labels:   map[string]string{"node-role.kubernetes.io/worker": ""},
			expected: "worker",
		},
		{
			name:     "no role labels",
			labels:   map[string]string{},
			expected: "worker",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeRole(tt.labels); got != tt.expected {
				t.Errorf("nodeRole() = %q, want %q", got, tt.expected)
			}
		})
	}
}
