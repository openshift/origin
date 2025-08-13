package admupgradestatus

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type wasUpdatedFn func() (bool, error)

func (w *monitor) updateLifecycle(wasUpdated wasUpdatedFn) *junitapi.JUnitTestCase {
	health := &junitapi.JUnitTestCase{
		Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status snapshots reflect the cluster upgrade lifecycle",
	}

	clusterUpdated, err := wasUpdated()
	if err != nil {
		health.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("failed to get cluster version: %v", err),
		}
		return health
	}

	health.SkipMessage = &junitapi.SkipMessage{
		Message: "Test skipped because no oc adm upgrade status output was successfully collected",
	}

	type state string
	const (
		beforeUpdate             state = "before update"
		controlPlaneUpdating     state = "control plane updating"
		controlPlaneNodesUpdated state = "control plane nodes updated"
		controlPlaneUpdated      state = "control plane updated"
		afterUpdate              state = "after update"
	)

	type observation string
	const (
		notUpdating                      observation = "not updating"
		controlPlaneObservedUpdating     observation = "control plane updating"
		controlPlaneObservedNodesUpdated observation = "control plane nodes updated"
		controlPlaneObservedUpdated      observation = "control plane updated"
	)

	stateTransitions := map[state]map[observation]state{
		beforeUpdate: {
			notUpdating:                      beforeUpdate,
			controlPlaneObservedUpdating:     controlPlaneUpdating,
			controlPlaneObservedNodesUpdated: controlPlaneNodesUpdated,
			controlPlaneObservedUpdated:      controlPlaneUpdated,
		},
		controlPlaneUpdating: {
			notUpdating:                      afterUpdate,
			controlPlaneObservedUpdating:     controlPlaneUpdating,
			controlPlaneObservedNodesUpdated: controlPlaneNodesUpdated,
			controlPlaneObservedUpdated:      controlPlaneUpdated,
		},
		controlPlaneNodesUpdated: {
			notUpdating:                      afterUpdate,
			controlPlaneObservedNodesUpdated: controlPlaneNodesUpdated,
			controlPlaneObservedUpdated:      controlPlaneUpdated,
		},
		controlPlaneUpdated: {
			notUpdating:                 afterUpdate,
			controlPlaneObservedUpdated: controlPlaneUpdated,
		},
		afterUpdate: {
			notUpdating: afterUpdate,
		},
	}

	current := beforeUpdate
	failureOutputBuilder := strings.Builder{}

	for _, observed := range w.ocAdmUpgradeStatusOutputModels {
		if observed.output == nil {
			// Failing to parse the output is handled in expectedLayout, so we can skip here
			continue
		}
		// We saw at least one successful execution of oc adm upgrade status, so we have data to process
		health.SkipMessage = nil

		wroteOnce := false
		fail := func(message string) {
			if !wroteOnce {
				wroteOnce = true
				failureOutputBuilder.WriteString(fmt.Sprintf("\n===== %s\n", observed.when.Format(time.RFC3339)))
				failureOutputBuilder.WriteString(observed.output.rawOutput)
				failureOutputBuilder.WriteString(fmt.Sprintf("\n\n=> %s\n", message))
			}
		}

		if !clusterUpdated {
			if observed.output.updating || observed.output.controlPlane != nil || observed.output.workers != nil || observed.output.health != nil {
				fail("Cluster did not update but oc adm upgrade status reported that it is updating")
			}
			continue
		}

		controlPlane := observed.output.controlPlane

		o := notUpdating
		switch {
		case controlPlane != nil && controlPlane.Updated:
			o = controlPlaneObservedUpdated
		case controlPlane != nil && controlPlane.NodesUpdated:
			o = controlPlaneObservedNodesUpdated
		case observed.output.updating:
			o = controlPlaneObservedUpdating
		}

		fromCurrent := stateTransitions[current]
		if next, ok := fromCurrent[o]; !ok {
			fail(fmt.Sprintf("Unexpected observation '%s' in state '%s'", o, current))
		} else {
			current = next
		}
	}

	if failureOutputBuilder.Len() > 0 {
		health.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("observed unexpected update lifecycle transition in oc adm upgrade status"),
			Output:  failureOutputBuilder.String(),
		}
	}

	return health
}
