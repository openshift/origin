package nodeprovisioninglatency

import (
	"testing"
	"time"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestCollectionStateRows(t *testing.T) {
	beginning := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	machineCreated := beginning.Add(time.Minute)
	nodeCreated := beginning.Add(3 * time.Minute)
	nodeReady := beginning.Add(4 * time.Minute)

	tests := []struct {
		name              string
		machine           *machinev1beta1.Machine
		node              *corev1.Node
		wantNodeName      string
		wantMachineToInst time.Duration
		wantMachineToRead time.Duration
	}{
		{
			name:    "correlates by nodeRef",
			machine: testMachine("worker-1", types.UID("uid-1"), machineCreated, &corev1.ObjectReference{Name: "node-1"}),
			node:    testNode("node-1", types.UID("node-uid-1"), nodeCreated, nodeReady, ""),
			// durations asserted below
			wantNodeName:      "node-1",
			wantMachineToInst: time.Minute,
			wantMachineToRead: 3 * time.Minute,
		},
		{
			name:              "falls back to the machine annotation",
			machine:           testMachine("worker-2", types.UID("uid-2"), machineCreated, nil),
			node:              testNode("node-2", types.UID("node-uid-2"), nodeCreated, nodeReady, "openshift-machine-api/worker-2"),
			wantNodeName:      "node-2",
			wantMachineToInst: time.Minute,
			wantMachineToRead: 3 * time.Minute,
		},
		{
			name:              "machine without a node still produces a row",
			machine:           testMachine("worker-3", types.UID("uid-3"), machineCreated, nil),
			wantNodeName:      "",
			wantMachineToInst: time.Minute,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := newCollectionState()
			state.observeMachine(test.machine)
			if test.node != nil {
				state.observeNode(test.node)
			}

			rows := state.rows(beginning)
			if len(rows) != 1 {
				t.Fatalf("expected 1 row, got %d", len(rows))
			}

			row := rows[0]
			if row.NodeName != test.wantNodeName {
				t.Errorf("NodeName = %q, want %q", row.NodeName, test.wantNodeName)
			}
			assertDuration(t, "MachineToInstance", row.MachineToInstance, durationPtr(test.wantMachineToInst))
			if test.wantMachineToRead == 0 {
				assertDuration(t, "MachineToReady", row.MachineToReady, nil)
			} else {
				assertDuration(t, "MachineToReady", row.MachineToReady, durationPtr(test.wantMachineToRead))
			}
		})
	}
}

func TestCollectionStateRetainsDeletedMachinesAndSortsRows(t *testing.T) {
	beginning := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	state := newCollectionState()

	second := testMachine("worker-b", types.UID("uid-b"), beginning.Add(time.Minute), nil)
	first := testMachine("worker-a", types.UID("uid-a"), beginning.Add(time.Minute), nil)
	state.observeMachine(second)
	state.observeMachine(first)
	state.observeMachineDeleted(second)

	rows := state.rows(beginning)
	if len(rows) != 2 {
		t.Fatalf("expected deleted Machine to be retained, got %d rows", len(rows))
	}
	if rows[0].MachineName != "worker-a" || rows[1].MachineName != "worker-b" {
		t.Fatalf("rows are not sorted by Machine name: %q, %q", rows[0].MachineName, rows[1].MachineName)
	}
	if !rows[1].MachineDeletedDuringRun {
		t.Errorf("expected worker-b to be marked deleted during the run")
	}
	if rows[0].MachineDeletedDuringRun {
		t.Errorf("expected worker-a not to be marked deleted")
	}
}

func TestObserveMachineConditionAndObservedFallback(t *testing.T) {
	machine := testMachine("worker-1", types.UID("uid-1"), time.Now(), nil)
	machine.Status.Conditions = []machinev1beta1.Condition{{
		Type:               machinev1beta1.InstanceExistsCondition,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Minute)),
	}}
	machine.Status.Phase = ptr.To(string(machinev1beta1.PhaseProvisioning))

	state := newCollectionState()
	state.observeMachine(machine)

	rec := state.machines[types.UID("uid-1")]
	if rec == nil {
		t.Fatal("expected machine record")
	}
	if !rec.hasInstanceExists {
		t.Errorf("expected InstanceExists to be recorded")
	}
	if rec.hasProvisionedObserved {
		t.Errorf("Provisioning phase must not set the observed fallback")
	}

	machine.Status.Phase = ptr.To(string(machinev1beta1.PhaseRunning))
	state.observeMachine(machine)
	if !state.machines[types.UID("uid-1")].hasProvisionedObserved {
		t.Errorf("expected Running phase to set the observed fallback")
	}
}

func testMachine(name string, uid types.UID, created time.Time, nodeRef *corev1.ObjectReference) *machinev1beta1.Machine {
	return &machinev1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			UID:               uid,
			CreationTimestamp: metav1.NewTime(created),
			Labels: map[string]string{
				machineRoleLabel: roleWorker,
				machineSetLabel:  "worker-" + name,
			},
		},
		Status: machinev1beta1.MachineStatus{
			Phase:      ptr.To(string(machinev1beta1.PhaseRunning)),
			NodeRef:    nodeRef,
			Conditions: []machinev1beta1.Condition{{Type: machinev1beta1.InstanceExistsCondition, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(created.Add(time.Minute))}},
		},
	}
}

func testNode(name string, uid types.UID, created, ready time.Time, machineName string) *corev1.Node {
	annotations := map[string]string{}
	if machineName != "" {
		annotations[machineAnnotation] = machineName
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			UID:               uid,
			CreationTimestamp: metav1.NewTime(created),
			Labels:            map[string]string{"node-role.kubernetes.io/worker": ""},
			Annotations:       annotations,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(ready)}},
		},
	}
}
