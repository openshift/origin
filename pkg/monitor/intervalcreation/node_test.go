package intervalcreation

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

func TestIntervalsFromEvents_NodeChanges(t *testing.T) {
	intervals, err := monitorserialization.EventsFromFile("testdata/node.json")
	if err != nil {
		t.Fatal(err)
	}
	events := make([]*monitorapi.Event, 0, len(intervals))
	for _, i := range intervals {
		events = append(events, &monitorapi.Event{
			Condition: i.Condition,
			At:        i.From,
		})
	}
	changes := IntervalsFromEvents_NodeChanges(events, time.Time{}, time.Now())
	out, _ := monitorserialization.EventsIntervalsToJSON(changes)
	if len(changes) != 3 {
		t.Fatalf("unexpected changes: %s", string(out))
	}
	if changes[0].Message != "reason/NodeUpdate phase/Drain drained node" {
		t.Errorf("unexpected event: %s", string(out))
	}
	if changes[1].Message != "reason/NodeUpdate phase/OperatingSystemUpdate updated operating system" {
		t.Errorf("unexpected event: %s", string(out))
	}
	if changes[2].Message != "reason/NodeUpdate phase/Reboot rebooted and kubelet started" {
		t.Errorf("unexpected event: %s", string(out))
	}
}
