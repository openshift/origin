package nodestateanalyzer

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/statetracker"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

const (
	msgPhaseDrain    = "phase/Drain roles/%s drained node"
	msgPhaseOSUpdate = "phase/OperatingSystemUpdate roles/%s updated operating system"
	msgPhaseReboot   = "phase/Reboot roles/%s rebooted and kubelet started"
)

func intervalsFromEvents_NodeChanges(events monitorapi.Intervals, _ monitorapi.ResourcesMap, beginning, end time.Time) monitorapi.Intervals {
	var intervals monitorapi.Intervals
	nodeStateTracker := statetracker.NewStateTracker(monitorapi.ConstructionOwnerNodeLifecycle, beginning)
	locatorToMessageAnnotations := map[string]map[string]string{}

	for _, event := range events {
		node, ok := monitorapi.NodeFromLocator(event.Locator)
		if !ok {
			continue
		}
		reason := monitorapi.ReasonFrom(event.Message)
		if len(reason) == 0 {
			continue
		}

		roles := monitorapi.GetNodeRoles(event)
		nodeLocator := monitorapi.NodeLocator(node)
		if _, ok := locatorToMessageAnnotations[nodeLocator]; !ok {
			locatorToMessageAnnotations[nodeLocator] = map[string]string{}
		}
		locatorToMessageAnnotations[nodeLocator]["role"] = roles

		notReadyState := statetracker.State("NotReady", monitorapi.NodeNotReadyReason)
		updateState := statetracker.State("Update", monitorapi.NodeUpdateReason)
		drainState := statetracker.State("Drain", monitorapi.NodeUpdateReason)
		osUpdateState := statetracker.State("OperatingSystemUpdate", monitorapi.NodeUpdateReason)
		rebootState := statetracker.State("Reboot", monitorapi.NodeUpdateReason)

		// Use the four key reasons set by the MCD in events - since events are best effort these
		// could easily be incorrect or hide problems like double invocations. For now use these as
		// timeline indicators only, and use the stronger observed State events from the node object.
		// A separate test should look for anomalies in these intervals if the need is identified.
		//
		// The current structure will hold events open until they are closed, so you would see
		// excessively long intervals if the events are missing (because OpenInterval does not create
		// a new interval).
		switch reason {
		case "NotReady":
			nodeStateTracker.OpenInterval(nodeLocator, notReadyState, event.From)
		case "Ready":
			message := monitorapi.NewMessage().Reason(monitorapi.NodeNotReadyReason).HumanMessagef("role/%v is not ready", roles).BuildString()
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, notReadyState, statetracker.SimpleCondition(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.Warning, monitorapi.NodeNotReadyReason, message), event.From)...)
		case "MachineConfigChange":
			nodeStateTracker.OpenInterval(nodeLocator, updateState, event.From)
		case "MachineConfigReached":
			message := strings.ReplaceAll(event.Message, "reason/MachineConfigReached ", "phase/Update ") + " roles/" + roles
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, updateState, statetracker.SimpleCondition(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.Info, monitorapi.NodeUpdateReason, message), event.From)...)
		case "Cordon", "Drain":
			nodeStateTracker.OpenInterval(nodeLocator, drainState, event.From)
		case "OSUpdateStarted":
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, drainState, statetracker.SimpleCondition(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseDrain, roles)), event.From)...)
			nodeStateTracker.OpenInterval(nodeLocator, osUpdateState, event.From)
		case "Reboot":
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, drainState, statetracker.SimpleCondition(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseDrain, roles)), event.From)...)
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, osUpdateState, statetracker.SimpleCondition(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseOSUpdate, roles)), event.From)...)
			nodeStateTracker.OpenInterval(nodeLocator, rebootState, event.From)
		case "Starting":
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, drainState, statetracker.SimpleCondition(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseDrain, roles)), event.From)...)
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, osUpdateState, statetracker.SimpleCondition(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseOSUpdate, roles)), event.From)...)
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, rebootState, statetracker.SimpleCondition(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseReboot, roles)), event.From)...)
		}
	}
	intervals = append(intervals, nodeStateTracker.CloseAllIntervals(locatorToMessageAnnotations, end)...)

	return intervals
}
