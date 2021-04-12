package intervalcreation

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func IntervalsFromEvents_NodeChanges(events []*monitorapi.Event, beginning, end time.Time) monitorapi.EventIntervals {
	var intervals monitorapi.EventIntervals
	nodeChangeToLastStart := map[string]map[string]time.Time{}

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
		intervals = append(intervals, &monitorapi.EventInterval{
			Condition: &monitorapi.Condition{
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
			openInterval(state, "Update", event.At)
		case "MachineConfigReached":
			closeInterval(state, "Update", monitorapi.Info, event.Locator, strings.ReplaceAll(event.Message, "reason/MachineConfigReached ", "reason/NodeUpdate phase/Update "), event.At)
		case "Cordon", "Drain":
			openInterval(state, "Drain", event.At)
		case "OSUpdateStarted":
			closeInterval(state, "Drain", monitorapi.Info, event.Locator, "reason/NodeUpdate phase/Drain drained node", event.At)
			openInterval(state, "OperatingSystemUpdate", event.At)
		case "Reboot":
			closeInterval(state, "Drain", monitorapi.Info, event.Locator, "reason/NodeUpdate phase/Drain drained node", event.At)
			closeInterval(state, "OperatingSystemUpdate", monitorapi.Info, event.Locator, "reason/NodeUpdate phase/OperatingSystemUpdate updated operating system", event.At)
			openInterval(state, "Reboot", event.At)
		case "Starting":
			closeInterval(state, "Drain", monitorapi.Info, event.Locator, "reason/NodeUpdate phase/Drain drained node", event.At)
			closeInterval(state, "OperatingSystemUpdate", monitorapi.Info, event.Locator, "reason/NodeUpdate phase/OperatingSystemUpdate updated operating system", event.At)
			closeInterval(state, "Reboot", monitorapi.Info, event.Locator, "reason/NodeUpdate phase/Reboot rebooted and kubelet started", event.At)
		}
	}

	for nodeName, state := range nodeChangeToLastStart {
		for name := range state {
			closeInterval(state, name, monitorapi.Warning, monitorapi.NodeLocator(nodeName), fmt.Sprintf("reason/NodeUpdate phase/%s phase never completed", name), end)
		}
	}

	return intervals
}
