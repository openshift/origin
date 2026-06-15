package clusterinstancetypes

import (
	"encoding/json"
	"testing"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func awsMachine(t *testing.T, name, role, instanceType string) machinev1beta1.Machine {
	t.Helper()
	providerSpec := machinev1beta1.AWSMachineProviderConfig{
		InstanceType: instanceType,
	}
	raw, err := json.Marshal(providerSpec)
	if err != nil {
		t.Fatalf("failed to marshal provider spec: %v", err)
	}
	return machinev1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"machine.openshift.io/cluster-api-machine-role": role},
		},
		Spec: machinev1beta1.MachineSpec{
			ProviderSpec: machinev1beta1.ProviderSpec{
				Value: &runtime.RawExtension{Raw: raw},
			},
		},
	}
}

func TestBuildRowsDeduplicates(t *testing.T) {
	machines := []machinev1beta1.Machine{
		awsMachine(t, "master-0", "master", "m6i.xlarge"),
		awsMachine(t, "master-1", "master", "m6i.xlarge"),
		awsMachine(t, "master-2", "master", "m6i.xlarge"),
		awsMachine(t, "worker-0", "worker", "m6i.2xlarge"),
		awsMachine(t, "worker-1", "worker", "m6i.2xlarge"),
		awsMachine(t, "worker-2", "worker", "m6i.2xlarge"),
	}

	rows := buildRows("aws", "us-east-1", machines)

	if len(rows) != 2 {
		t.Fatalf("expected 2 deduplicated rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].Role != "control-plane" || rows[0].InstanceType != "m6i.xlarge" {
		t.Errorf("unexpected control-plane row: %+v", rows[0])
	}
	if rows[1].Role != "worker" || rows[1].InstanceType != "m6i.2xlarge" {
		t.Errorf("unexpected worker row: %+v", rows[1])
	}
}

func TestBuildRowsMixedWorkerTypes(t *testing.T) {
	machines := []machinev1beta1.Machine{
		awsMachine(t, "master-0", "master", "m6i.xlarge"),
		awsMachine(t, "worker-0", "worker", "m5.xlarge"),
		awsMachine(t, "worker-1", "worker", "m6i.2xlarge"),
		awsMachine(t, "worker-2", "worker", "m5.xlarge"),
	}

	rows := buildRows("aws", "us-east-1", machines)

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
	machines := []machinev1beta1.Machine{
		awsMachine(t, "worker-0", "worker", "m5.xlarge"),
		awsMachine(t, "master-0", "master", "m6i.xlarge"),
	}

	rows := buildRows("aws", "us-east-1", machines)

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Role != "control-plane" {
		t.Errorf("control-plane should sort first, got %+v", rows[0])
	}
}

func TestBuildRowsPropagatesPlatformAndRegion(t *testing.T) {
	machines := []machinev1beta1.Machine{
		awsMachine(t, "master-0", "master", "m6i.xlarge"),
	}

	rows := buildRows("aws", "eu-west-1", machines)

	if rows[0].Platform != "aws" || rows[0].Region != "eu-west-1" {
		t.Errorf("expected platform=aws region=eu-west-1, got %+v", rows[0])
	}
}

func TestBuildRowsSkipsEmptyProviderSpec(t *testing.T) {
	machines := []machinev1beta1.Machine{
		awsMachine(t, "master-0", "master", "m6i.xlarge"),
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "worker-no-spec",
				Labels: map[string]string{"machine.openshift.io/cluster-api-machine-role": "worker"},
			},
		},
	}

	rows := buildRows("aws", "us-east-1", machines)

	if len(rows) != 1 {
		t.Fatalf("expected 1 row (worker with no spec skipped), got %d: %+v", len(rows), rows)
	}
}
