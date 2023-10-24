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
	nodeStateTracker := statetracker.NewStateTracker(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.SourceNodeState, beginning)
	locatorToMessageAnnotations := map[string]map[string]string{}

	for _, event := range events {
		// TODO: dangerous assumptions here without using interval source, we ended up picking up container
		// ready events because they have a node in the locator, and a reason of "Ready".
		// Once the reasons marked "not ported" in the comments below are ported, we could filter here on
		// event.Source to ensure we only look at what we intend.
		node, ok := monitorapi.NodeFromLocator(event.Locator)
		if !ok {
			continue
		}
		reason := monitorapi.ReasonFrom(event.Message)
		if len(reason) == 0 {
			continue
		}

		roles := monitorapi.GetNodeRoles(event)

		nodeLocator := monitorapi.NewLocator().NodeFromName(node)
		nodeLocatorKey := nodeLocator.OldLocator()
		if _, ok := locatorToMessageAnnotations[nodeLocatorKey]; !ok {
			locatorToMessageAnnotations[nodeLocatorKey] = map[string]string{}
		}
		locatorToMessageAnnotations[nodeLocatorKey]["role"] = roles

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
			if event.Source == monitorapi.SourceNodeMonitor {
				nodeStateTracker.OpenInterval(nodeLocator, notReadyState, event.From)
			}
		case "Ready":
			if event.Source == monitorapi.SourceNodeMonitor {
				message := monitorapi.NewMessage().Reason(monitorapi.NodeNotReadyReason).HumanMessagef("role/%v node is not ready", roles).BuildString()
				intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, notReadyState, statetracker.SimpleInterval(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.SourceNodeState, monitorapi.Warning, monitorapi.NodeNotReadyReason, message), event.From)...)
			}
		case "MachineConfigChange":
			if event.Source == monitorapi.SourceNodeMonitor {
				nodeStateTracker.OpenInterval(nodeLocator, updateState, event.From)
			}
		case "MachineConfigReached":
			if event.Source == monitorapi.SourceNodeMonitor {
				message := strings.ReplaceAll(event.Message, "reason/MachineConfigReached ", "phase/Update ") + " roles/" + roles
				intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, updateState, statetracker.SimpleInterval(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.SourceNodeState, monitorapi.Info, monitorapi.NodeUpdateReason, message), event.From)...)
			}
		case "Cordon", "Drain":
			// Not ported, so we don't have a Source to check
			nodeStateTracker.OpenInterval(nodeLocator, drainState, event.From)
		case "OSUpdateStarted":
			// Not ported, so we don't have a Source to check
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, drainState, statetracker.SimpleInterval(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.SourceNodeState, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseDrain, roles)), event.From)...)
			nodeStateTracker.OpenInterval(nodeLocator, osUpdateState, event.From)
		case "Reboot":
			// Not ported, so we don't have a Source to check
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, drainState, statetracker.SimpleInterval(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.SourceNodeState, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseDrain, roles)), event.From)...)
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, osUpdateState, statetracker.SimpleInterval(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.SourceNodeState, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseOSUpdate, roles)), event.From)...)
			nodeStateTracker.OpenInterval(nodeLocator, rebootState, event.From)
		case "Starting":
			// Not ported, so we don't have a Source to check
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, drainState, statetracker.SimpleInterval(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.SourceNodeState, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseDrain, roles)), event.From)...)
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, osUpdateState, statetracker.SimpleInterval(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.SourceNodeState, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseOSUpdate, roles)), event.From)...)
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, rebootState, statetracker.SimpleInterval(monitorapi.ConstructionOwnerNodeLifecycle, monitorapi.SourceNodeState, monitorapi.Info, monitorapi.NodeUpdateReason, fmt.Sprintf(msgPhaseReboot, roles)), event.From)...)
		}
	}
	intervals = append(intervals, nodeStateTracker.CloseAllIntervals(locatorToMessageAnnotations, end)...)

	return intervals
}
