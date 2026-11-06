package allowednodeprovisioning

import (
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

func TestDataKeyName(t *testing.T) {
	tests := []struct {
		name             string
		segment          string
		role             string
		createdDuringRun bool
		want             string
	}{
		{name: "created during run", segment: "MachineToReadySeconds", role: "worker", createdDuringRun: true, want: "MachineToReadySeconds/worker/true"},
		{name: "pre-existing machine", segment: "NodeToReadySeconds", role: "master", createdDuringRun: false, want: "NodeToReadySeconds/master/false"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DataKeyName(test.segment, test.role, test.createdDuringRun); got != test.want {
				t.Errorf("DataKeyName() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEmptyBaselineSkipsInsteadOfFailing(t *testing.T) {
	jobType := platformidentification.JobType{Release: "4.22", Platform: "aws", Architecture: "amd64", Network: "OVNKubernetes", Topology: "ha"}

	p99, _, err := GetAllowedProvisioningLatency("MachineToReadySeconds", "worker", true, jobType)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p99 != nil {
		t.Fatalf("expected no historical data, got %v", *p99)
	}
}

func TestUnknownJobTypeSkipsWithoutPanicking(t *testing.T) {
	tests := []struct {
		name    string
		jobType platformidentification.JobType
	}{
		{name: "no platform", jobType: platformidentification.JobType{Release: "4.22"}},
		{name: "no release", jobType: platformidentification.JobType{Platform: "aws"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p99, _, err := GetAllowedProvisioningLatency("MachineToReadySeconds", "worker", true, test.jobType)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p99 != nil {
				t.Fatalf("expected no baseline, got %v", *p99)
			}
		})
	}
}

func TestQueryEmitsTheDataKeyNameShape(t *testing.T) {
	if !strings.Contains(provisioningP99ViewQuery, "ci_data_autodl.node_provisioning_latency") {
		t.Errorf("query does not read the node provisioning autodl table")
	}
	if !strings.Contains(provisioningP99ViewQuery, "CONCAT(Segment, '/', MachineRole, '/', CreatedDuringRun) AS BackendName") {
		t.Errorf("query BackendName must match DataKeyName")
	}
}
