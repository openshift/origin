package nodeprovisioninglatency

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	machineAPINamespace = "openshift-machine-api"

	machineRoleLabel  = "machine.openshift.io/cluster-api-machine-role"
	machineSetLabel   = "machine.openshift.io/cluster-api-machineset"
	machineAnnotation = "machine.openshift.io/machine"

	roleMaster  = "master"
	roleWorker  = "worker"
	roleUnknown = "unknown"

	sourceCondition = "condition"
	sourceObserved  = "observed"
	sourceMissing   = "missing"

	segmentMachineToInstance = "MachineToInstanceSeconds"
	segmentInstanceToNode    = "InstanceToNodeSeconds"
	segmentNodeToReady       = "NodeToReadySeconds"
	segmentMachineToReady    = "MachineToReadySeconds"
	segmentMachineToNode     = "MachineToNodeSeconds"
)

// machineRecord is the accumulated view of one Machine. Records are kept after the Machine is
// deleted so Machines that are created and torn down during a run still produce a row.
type machineRecord struct {
	UID          types.UID
	Name         string
	Creation     time.Time
	Phase        string
	RoleLabel    string
	MachineSet   string
	InstanceType string
	NodeRefName  string

	instanceExistsTime     time.Time
	hasInstanceExists      bool
	provisionedObserved    time.Time
	hasProvisionedObserved bool

	deleted     bool
	deletedTime time.Time
}

// nodeRecord is the accumulated view of one Node. Kept after deletion for Machine correlation.
type nodeRecord struct {
	UID       types.UID
	Name      string
	Creation  time.Time
	RoleLabel string

	readyTime        time.Time
	hasReady         bool
	readyObserved    time.Time
	hasReadyObserved bool

	machineAnnotation string
	deleted           bool
}

// provisioningRow is one autodl row: one observed Machine.
type provisioningRow struct {
	MachineName  string
	MachineSet   string
	MachineRole  string
	InstanceType string
	MachinePhase string

	CreatedDuringRun        bool
	MachineDeletedDuringRun bool

	MachineCreationTimestamp time.Time
	InstanceCreatedTimestamp time.Time
	InstanceExistsSource     string
	NodeCreationTimestamp    time.Time
	NodeReadyTimestamp       time.Time
	NodeReadySource          string
	NodeName                 string

	MachineToInstance *time.Duration
	InstanceToNode    *time.Duration
	NodeToReady       *time.Duration
	MachineToNode     *time.Duration
	MachineToReady    *time.Duration
}

func findCondition(conditions []machinev1beta1.Condition, conditionType machinev1beta1.ConditionType) *machinev1beta1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func nodeReadyCondition(node *corev1.Node) *corev1.NodeCondition {
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == corev1.NodeReady {
			return &node.Status.Conditions[i]
		}
	}
	return nil
}

func nodeRoleFromLabels(labels map[string]string) string {
	if _, ok := labels["node-role.kubernetes.io/master"]; ok {
		return roleMaster
	}
	if _, ok := labels["node-role.kubernetes.io/control-plane"]; ok {
		return roleMaster
	}
	if _, ok := labels["node-role.kubernetes.io/worker"]; ok {
		return roleWorker
	}
	return roleUnknown
}

func normalizeRole(machineRole string, node *nodeRecord) string {
	switch machineRole {
	case roleMaster, "control-plane":
		return roleMaster
	case roleWorker:
		return roleWorker
	}
	if node != nil && node.RoleLabel != "" {
		return node.RoleLabel
	}
	return roleUnknown
}

// instanceTypeFromMachine reads the provider-specific instance type best-effort. AWS, GCP and
// Azure use different field names for the same concept.
func instanceTypeFromMachine(m *machinev1beta1.Machine) string {
	if m.Spec.ProviderSpec.Value == nil || len(m.Spec.ProviderSpec.Value.Raw) == 0 {
		return ""
	}
	providerSpec := map[string]interface{}{}
	if err := json.Unmarshal(m.Spec.ProviderSpec.Value.Raw, &providerSpec); err != nil {
		return ""
	}
	for _, key := range []string{"instanceType", "machineType", "vmSize"} {
		if value, ok := providerSpec[key].(string); ok {
			return value
		}
	}
	return ""
}

// machineNameFromAnnotation extracts the Machine name from a node's
// machine.openshift.io/machine annotation, whose value is "<namespace>/<name>".
func machineNameFromAnnotation(annotation string) string {
	if idx := strings.LastIndex(annotation, "/"); idx >= 0 {
		return annotation[idx+1:]
	}
	return annotation
}

// secondsBetween returns nil when either endpoint is missing or out of order. Out-of-order
// timestamps are rejected rather than clamped so bad data is visible as a missing value.
func secondsBetween(start, end time.Time) *time.Duration {
	if start.IsZero() || end.IsZero() {
		return nil
	}
	duration := end.Sub(start)
	if duration < 0 {
		return nil
	}
	return &duration
}

func createdDuringRun(creation, beginning time.Time) bool {
	if creation.IsZero() {
		return false
	}
	return !creation.Before(beginning)
}

// buildRow derives all timestamps and durations for one Machine. beginning bounds the run; node
// may be nil when no Node could be correlated.
func buildRow(rec *machineRecord, node *nodeRecord, beginning time.Time) provisioningRow {
	duringRun := createdDuringRun(rec.Creation, beginning)

	instanceTime := rec.instanceExistsTime
	instanceSource := sourceMissing
	switch {
	case rec.hasInstanceExists && !rec.instanceExistsTime.IsZero():
		instanceSource = sourceCondition
	case duringRun && rec.hasProvisionedObserved:
		instanceTime = rec.provisionedObserved
		instanceSource = sourceObserved
	}

	var nodeCreation, nodeReady time.Time
	nodeReadySource := sourceMissing
	nodeName := ""
	if node != nil {
		nodeName = node.Name
		nodeCreation = node.Creation
		switch {
		case node.hasReady:
			nodeReady = node.readyTime
			nodeReadySource = sourceCondition
		case createdDuringRun(node.Creation, beginning) && node.hasReadyObserved:
			nodeReady = node.readyObserved
			nodeReadySource = sourceObserved
		}
	}

	phase := rec.Phase
	if phase == "" {
		phase = "Unknown"
	}

	return provisioningRow{
		MachineName:              rec.Name,
		MachineSet:               rec.MachineSet,
		MachineRole:              normalizeRole(rec.RoleLabel, node),
		InstanceType:             rec.InstanceType,
		MachinePhase:             phase,
		CreatedDuringRun:         duringRun,
		MachineDeletedDuringRun:  rec.deleted && !rec.deletedTime.Before(beginning),
		MachineCreationTimestamp: rec.Creation,
		InstanceCreatedTimestamp: instanceTime,
		InstanceExistsSource:     instanceSource,
		NodeCreationTimestamp:    nodeCreation,
		NodeReadyTimestamp:       nodeReady,
		NodeReadySource:          nodeReadySource,
		NodeName:                 nodeName,
		MachineToInstance:        secondsBetween(rec.Creation, instanceTime),
		InstanceToNode:           secondsBetween(instanceTime, nodeCreation),
		NodeToReady:              secondsBetween(nodeCreation, nodeReady),
		MachineToNode:            secondsBetween(rec.Creation, nodeCreation),
		MachineToReady:           secondsBetween(rec.Creation, nodeReady),
	}
}

// installerCreatedMaster reports whether the row describes a master that already existed when the
// run began. Those Machines belong to the install rather than to Machine API provisioning, so
// including them would compare install time against provisioning time.
func installerCreatedMaster(row provisioningRow) bool {
	return row.MachineRole == roleMaster && !row.CreatedDuringRun
}

// medianDuration returns the middle value, averaging the two middle values for an even count.
func medianDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}
