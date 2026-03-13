package extensions

import "strings"

// AddConflicts applies conflict annotations to tests that should not run in parallel
// with each other. Tests sharing the same conflict string will be serialized relative
// to each other but can still run in parallel with other tests.
func AddConflicts(specs ExtensionTestSpecs) {
	addStatefulSetPVCConflicts(specs)
}

// addStatefulSetPVCConflicts prevents StatefulSet tests that create large PVCs from
// running concurrently. These tests request 10Gi PVCs, and on environments with small
// disks (e.g. MicroShift with 20GB), running multiple of them in parallel can exhaust
// disk space.
func addStatefulSetPVCConflicts(specs ExtensionTestSpecs) {
	const conflict = "statefulset-large-pvc"
	for _, spec := range specs {
		if strings.Contains(spec.Name, "StatefulSet Basic StatefulSet functionality") ||
			strings.Contains(spec.Name, "StatefulSet Non-retain") {
			spec.Resources.Isolation.Conflict = append(spec.Resources.Isolation.Conflict, conflict)
		}
	}
}
