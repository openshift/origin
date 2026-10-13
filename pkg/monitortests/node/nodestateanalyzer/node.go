package nodestateanalyzer

import (
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/statetracker"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

const (
	msgPhaseDrain    = "drained node"
	msgPhaseOSUpdate = "updated operating system"
	msgPhaseReboot   = "rebooted and kubelet started"
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
		node, ok := event.Locator.Keys[monitorapi.LocatorNodeKey]
		if !ok {
			continue
		}
		reason := event.Message.Reason
		if len(reason) == 0 {
			continue
		}

		roles := monitorapi.GetNodeRoles(event)

		nodeLocator := monitorapi.NewLocator().NodeFromName(node)
		nodeLocatorKey := nodeLocator.OldLocator()
		if _, ok := locatorToMessageAnnotations[nodeLocatorKey]; !ok {
			locatorToMessageAnnotations[nodeLocatorKey] = map[string]string{}
		}
		locatorToMessageAnnotations[nodeLocatorKey][string(monitorapi.AnnotationRoles)] = roles

		notReadyState := statetracker.State("NotReady", "NodeNotReady", monitorapi.NodeNotReadyReason)
		updateState := statetracker.State("Update", "NodeUpdate", monitorapi.NodeUpdateReason)
		drainState := statetracker.State("Drain", "NodeUpdatePhases", monitorapi.NodeUpdateReason)
		osUpdateState := statetracker.State("OperatingSystemUpdate", "NodeUpdatePhases", monitorapi.NodeUpdateReason)
		rebootState := statetracker.State("Reboot", "NodeUpdatePhases", monitorapi.NodeUpdateReason)

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
		case "Ready", monitorapi.NodeDeleted:
			if event.Source == monitorapi.SourceNodeMonitor {
				mb := monitorapi.NewMessage().Reason(monitorapi.NodeNotReadyReason).
					HumanMessage("node is not ready").
					WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
					WithAnnotation(monitorapi.AnnotationRoles, roles)
				intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, notReadyState,
					statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Warning, mb),
					event.From)...)
			}
		case "MachineConfigChange":
			if event.Source == monitorapi.SourceNodeMonitor {
				nodeStateTracker.OpenInterval(nodeLocator, updateState, event.From)
			}
		case "MachineConfigReached":
			if event.Source == monitorapi.SourceNodeMonitor {
				mb := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
					HumanMessage(event.Message.HumanMessage). // re-use the human message from the MachineConfigReached event
					WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
					WithAnnotation(monitorapi.AnnotationRoles, roles).
					WithAnnotation(monitorapi.AnnotationPhase, "Update")
				intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, updateState,
					statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, mb),
					event.From)...)
			}
		case "Cordon", "Drain":
			// Not ported, so we don't have a Source to check
			nodeStateTracker.OpenInterval(nodeLocator, drainState, event.From)
		case "OSUpdateStarted":
			// Not ported, so we don't have a Source to check
			mb := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
				HumanMessage(msgPhaseDrain).
				WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
				WithAnnotation(monitorapi.AnnotationRoles, roles).
				WithAnnotation(monitorapi.AnnotationPhase, "Drain")
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, drainState,
				statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, mb),
				event.From)...)
			nodeStateTracker.OpenInterval(nodeLocator, osUpdateState, event.From)
		case "Reboot":
			// Not ported, so we don't have a Source to check
			mb := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
				HumanMessage(msgPhaseDrain).
				WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
				WithAnnotation(monitorapi.AnnotationRoles, roles).
				WithAnnotation(monitorapi.AnnotationPhase, "Drain")
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, drainState,
				statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, mb),
				event.From)...)

			osUpdateMB := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
				HumanMessage(msgPhaseOSUpdate).
				WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
				WithAnnotation(monitorapi.AnnotationRoles, roles).
				WithAnnotation(monitorapi.AnnotationPhase, "OperatingSystemUpdate")
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, osUpdateState,
				statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, osUpdateMB),
				event.From)...)
			nodeStateTracker.OpenInterval(nodeLocator, rebootState, event.From)
		case "Starting":
			// Not ported, so we don't have a Source to check
			mb := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
				HumanMessage(msgPhaseDrain).
				WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
				WithAnnotation(monitorapi.AnnotationRoles, roles).
				WithAnnotation(monitorapi.AnnotationPhase, "Drain")
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, drainState,
				statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, mb),
				event.From)...)

			osUpdateMB := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
				HumanMessage(msgPhaseOSUpdate).
				WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
				WithAnnotation(monitorapi.AnnotationRoles, roles).
				WithAnnotation(monitorapi.AnnotationPhase, "OperatingSystemUpdate")
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, osUpdateState,
				statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, osUpdateMB),
				event.From)...)

			rebootMB := monitorapi.NewMessage().Reason(monitorapi.NodeUpdateReason).
				HumanMessage(msgPhaseReboot).
				WithAnnotation(monitorapi.AnnotationConstructed, monitorapi.ConstructionOwnerNodeLifecycle).
				WithAnnotation(monitorapi.AnnotationRoles, roles).
				WithAnnotation(monitorapi.AnnotationPhase, "Reboot")
			intervals = append(intervals, nodeStateTracker.CloseIfOpenedInterval(nodeLocator, rebootState,
				statetracker.SimpleInterval(monitorapi.SourceNodeState, monitorapi.Info, rebootMB),
				event.From)...)
		}
	}
	// Close all node intervals left hanging open:
	intervals = append(intervals, nodeStateTracker.CloseAllIntervals(locatorToMessageAnnotations, end)...)

	return intervals
}
