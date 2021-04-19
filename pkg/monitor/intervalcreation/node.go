package intervalcreation

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

const (
	msgPhaseDrain          = "reason/NodeUpdate phase/Drain roles/%s drained node"
	msgPhaseOSUpdate       = "reason/NodeUpdate phase/OperatingSystemUpdate roles/%s updated operating system"
	msgPhaseReboot         = "reason/NodeUpdate phase/Reboot roles/%s rebooted and kubelet started"
	msgPhaseNeverCompleted = "reason/NodeUpdate phase/%s roles/%s phase never completed"
)

func IntervalsFromEvents_NodeChanges(events monitorapi.Intervals, beginning, end time.Time) monitorapi.Intervals {
	var intervals monitorapi.Intervals
	nodeChangeToLastStart := map[string]map[string]time.Time{}
	nodeNameToRoles := map[string]string{}

	openInterval := func(state map[string]time.Time, name string, from time.Time) bool {
		if _, ok := state[name]; !ok {
			state[name] = from
			return false
		}
		return true
	}
	closeInterval := func(state map[string]time.Time, name string, level monitorapi.EventLevel, locator, message string, to time.Time) {
		from, ok := state[name]
		if !ok {
			return
		}
		intervals = append(intervals, monitorapi.EventInterval{
			Condition: monitorapi.Condition{
				Level:   level,
				Locator: locator,
				Message: message,
			},
			From: from,
			To:   to,
		})
		delete(state, name)
	}

	for _, event := range events {
		node, ok := monitorapi.NodeFromLocator(event.Locator)
		if !ok {
			continue
		}
		if !strings.HasPrefix(event.Message, "reason/") {
			continue
		}
		reason := strings.SplitN(strings.TrimPrefix(event.Message, "reason/"), " ", 2)[0]

		roles := monitorapi.GetNodeRoles(event)
		nodeNameToRoles[node] = roles
		state, ok := nodeChangeToLastStart[node]
		if !ok {
			state = make(map[string]time.Time)
			nodeChangeToLastStart[node] = state
		}

		// Use the four key reasons set by the MCD in events - since events are best effort these
		// could easily be incorrect or hide problems like double invocations. For now use these as
		// timeline indicators only, and use the stronger observed state events from the node object.
		// A separate test should look for anomalies in these intervals if the need is identified.
		//
		// The current structure will hold events open until they are closed, so you would see
		// excessively long intervals if the events are missing (because openInterval does not create
		// a new interval).
		switch reason {
		case "MachineConfigChange":
			openInterval(state, "Update", event.From)
		case "MachineConfigReached":
			message := strings.ReplaceAll(event.Message, "reason/MachineConfigReached ", "reason/NodeUpdate phase/Update ") + " roles/" + roles
			closeInterval(state, "Update", monitorapi.Info, event.Locator, message, event.From)
		case "Cordon", "Drain":
			openInterval(state, "Drain", event.From)
		case "OSUpdateStarted":
			closeInterval(state, "Drain", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseDrain, roles), event.From)
			openInterval(state, "OperatingSystemUpdate", event.From)
		case "Reboot":
			closeInterval(state, "Drain", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseDrain, roles), event.From)
			closeInterval(state, "OperatingSystemUpdate", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseOSUpdate, roles), event.From)
			openInterval(state, "Reboot", event.From)
		case "Starting":
			closeInterval(state, "Drain", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseDrain, roles), event.From)
			closeInterval(state, "OperatingSystemUpdate", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseOSUpdate, roles), event.From)
			closeInterval(state, "Reboot", monitorapi.Info, event.Locator, fmt.Sprintf(msgPhaseReboot, roles), event.From)
		}
	}

	for nodeName, state := range nodeChangeToLastStart {
		for name := range state {
			closeInterval(state, name, monitorapi.Warning, monitorapi.NodeLocator(nodeName), fmt.Sprintf(msgPhaseNeverCompleted, name, nodeNameToRoles[nodeName]), end)
		}
	}

	return intervals
}
