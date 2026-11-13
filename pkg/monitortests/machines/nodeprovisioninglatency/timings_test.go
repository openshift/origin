package nodeprovisioninglatency

import (
	"testing"
	"time"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

func machineFromJSON(t *testing.T, raw string) *machinev1beta1.Machine {
	t.Helper()
	return &machinev1beta1.Machine{
		Spec: machinev1beta1.MachineSpec{
			ProviderSpec: machinev1beta1.ProviderSpec{
				Value: &runtime.RawExtension{Raw: []byte(raw)},
			},
		},
	}
}

func TestBuildRow(t *testing.T) {
	beginning := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	machineCreated := beginning.Add(1 * time.Minute)
	instanceCreated := beginning.Add(2 * time.Minute)
	nodeCreated := beginning.Add(3 * time.Minute)
	nodeReady := beginning.Add(5 * time.Minute)

	tests := []struct {
		name               string
		machine            *machineRecord
		node               *nodeRecord
		wantInstanceSource string
		wantReadySource    string
		wantRole           string
		wantDuringRun      bool
		wantMachineToInst  *time.Duration
		wantInstToNode     *time.Duration
		wantNodeToReady    *time.Duration
		wantMachineToReady *time.Duration
	}{
		{
			name: "complete machine created during run",
			machine: &machineRecord{
				Name:               "worker-1",
				Creation:           machineCreated,
				Phase:              "Running",
				RoleLabel:          "worker",
				instanceExistsTime: instanceCreated,
				hasInstanceExists:  true,
			},
			node: &nodeRecord{
				Name:      "worker-1",
				Creation:  nodeCreated,
				readyTime: nodeReady,
				hasReady:  true,
			},
			wantInstanceSource: sourceCondition,
			wantReadySource:    sourceCondition,
			wantRole:           roleWorker,
			wantDuringRun:      true,
			wantMachineToInst:  durationPtr(time.Minute),
			wantInstToNode:     durationPtr(time.Minute),
			wantNodeToReady:    durationPtr(2 * time.Minute),
			wantMachineToReady: durationPtr(4 * time.Minute),
		},
		{
			name: "pre-existing machine without the condition has no instance time",
			machine: &machineRecord{
				Name:      "master-0",
				Creation:  beginning.Add(-24 * time.Hour),
				Phase:     "Running",
				RoleLabel: "master",
			},
			node: &nodeRecord{
				Name:      "master-0",
				Creation:  nodeCreated,
				readyTime: nodeReady,
				hasReady:  true,
			},
			wantInstanceSource: sourceMissing,
			wantReadySource:    sourceCondition,
			wantRole:           roleMaster,
			wantDuringRun:      false,
			wantNodeToReady:    durationPtr(2 * time.Minute),
			wantMachineToReady: durationPtr(nodeReady.Sub(beginning.Add(-24 * time.Hour))),
		},
		{
			name: "observed phase fallback only applies to machines created during the run",
			machine: &machineRecord{
				Name:                   "worker-2",
				Creation:               machineCreated,
				Phase:                  "Provisioned",
				RoleLabel:              "worker",
				provisionedObserved:    instanceCreated,
				hasProvisionedObserved: true,
			},
			wantInstanceSource: sourceObserved,
			wantReadySource:    sourceMissing,
			wantRole:           roleWorker,
			wantDuringRun:      true,
			wantMachineToInst:  durationPtr(time.Minute),
		},
		{
			name: "negative duration is rejected",
			machine: &machineRecord{
				Name:               "worker-3",
				Creation:           machineCreated,
				Phase:              "Running",
				RoleLabel:          "worker",
				instanceExistsTime: instanceCreated,
				hasInstanceExists:  true,
			},
			node: &nodeRecord{
				Name:     "worker-3",
				Creation: beginning, // before the instance exists
			},
			wantInstanceSource: sourceCondition,
			wantReadySource:    sourceMissing,
			wantRole:           roleWorker,
			wantDuringRun:      true,
			wantMachineToInst:  durationPtr(time.Minute),
		},
		{
			name: "ready observation fallback applies to nodes created during the run",
			machine: &machineRecord{
				Name:      "worker-4",
				Creation:  machineCreated,
				Phase:     "Running",
				RoleLabel: "worker",
			},
			node: &nodeRecord{
				Name:             "worker-4",
				Creation:         nodeCreated,
				readyObserved:    nodeReady,
				hasReadyObserved: true,
			},
			wantInstanceSource: sourceMissing,
			wantReadySource:    sourceObserved,
			wantRole:           roleWorker,
			wantDuringRun:      true,
			wantNodeToReady:    durationPtr(2 * time.Minute),
			wantMachineToReady: durationPtr(4 * time.Minute),
		},
		{
			name: "role falls back to the node role",
			machine: &machineRecord{
				Name:     "unnamed-role",
				Creation: machineCreated,
			},
			node: &nodeRecord{
				Name:      "unnamed-role",
				Creation:  nodeCreated,
				RoleLabel: roleMaster,
			},
			wantInstanceSource: sourceMissing,
			wantReadySource:    sourceMissing,
			wantRole:           roleMaster,
			wantDuringRun:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := buildRow(test.machine, test.node, beginning)

			if row.MachineRole != test.wantRole {
				t.Errorf("MachineRole = %q, want %q", row.MachineRole, test.wantRole)
			}
			if row.CreatedDuringRun != test.wantDuringRun {
				t.Errorf("CreatedDuringRun = %t, want %t", row.CreatedDuringRun, test.wantDuringRun)
			}
			if row.InstanceExistsSource != test.wantInstanceSource {
				t.Errorf("InstanceExistsSource = %q, want %q", row.InstanceExistsSource, test.wantInstanceSource)
			}
			if row.NodeReadySource != test.wantReadySource {
				t.Errorf("NodeReadySource = %q, want %q", row.NodeReadySource, test.wantReadySource)
			}
			assertDuration(t, "MachineToInstance", row.MachineToInstance, test.wantMachineToInst)
			assertDuration(t, "InstanceToNode", row.InstanceToNode, test.wantInstToNode)
			assertDuration(t, "NodeToReady", row.NodeToReady, test.wantNodeToReady)
			assertDuration(t, "MachineToReady", row.MachineToReady, test.wantMachineToReady)
		})
	}
}

func TestMedianDuration(t *testing.T) {
	tests := []struct {
		name   string
		values []time.Duration
		want   time.Duration
	}{
		{name: "empty", values: nil, want: 0},
		{name: "single", values: []time.Duration{5 * time.Second}, want: 5 * time.Second},
		{name: "odd", values: []time.Duration{30 * time.Second, 10 * time.Second, 20 * time.Second}, want: 20 * time.Second},
		{name: "even", values: []time.Duration{40 * time.Second, 10 * time.Second, 30 * time.Second, 20 * time.Second}, want: 25 * time.Second},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := medianDuration(test.values); got != test.want {
				t.Errorf("medianDuration(%v) = %v, want %v", test.values, got, test.want)
			}
		})
	}
}

func TestInstanceTypeFromMachine(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "aws", raw: `{"instanceType":"m5.large"}`, want: "m5.large"},
		{name: "gcp", raw: `{"machineType":"n2-standard-4"}`, want: "n2-standard-4"},
		{name: "azure", raw: `{"vmSize":"Standard_D4s_v3"}`, want: "Standard_D4s_v3"},
		{name: "missing", raw: `{}`, want: ""},
		{name: "invalid", raw: `not-json`, want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			machine := machineFromJSON(t, test.raw)
			if got := instanceTypeFromMachine(machine); got != test.want {
				t.Errorf("instanceTypeFromMachine() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestMachineNameFromAnnotation(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		want       string
	}{
		{name: "namespace and name", annotation: "openshift-machine-api/worker-1", want: "worker-1"},
		{name: "bare name", annotation: "worker-1", want: "worker-1"},
		{name: "empty", annotation: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := machineNameFromAnnotation(test.annotation); got != test.want {
				t.Errorf("machineNameFromAnnotation(%q) = %q, want %q", test.annotation, got, test.want)
			}
		})
	}
}

func TestInstallerCreatedMaster(t *testing.T) {
	tests := []struct {
		name string
		row  provisioningRow
		want bool
	}{
		{name: "master that predates the run", row: provisioningRow{MachineRole: roleMaster}, want: true},
		{name: "master created during the run", row: provisioningRow{MachineRole: roleMaster, CreatedDuringRun: true}, want: false},
		{name: "worker that predates the run", row: provisioningRow{MachineRole: roleWorker}, want: false},
		{name: "unknown role", row: provisioningRow{MachineRole: roleUnknown}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := installerCreatedMaster(test.row); got != test.want {
				t.Errorf("installerCreatedMaster(%+v) = %t, want %t", test.row, got, test.want)
			}
		})
	}
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func assertDuration(t *testing.T, name string, got, want *time.Duration) {
	t.Helper()
	if got == nil || want == nil {
		if got != nil || want != nil {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
		return
	}
	if *got != *want {
		t.Errorf("%s = %v, want %v", name, *got, *want)
	}
}
