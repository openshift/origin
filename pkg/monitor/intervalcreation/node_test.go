package intervalcreation

import (
	"testing"
	"time"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

func TestIntervalsFromEvents_NodeChanges(t *testing.T) {
	intervals, err := monitorserialization.EventsFromFile("testdata/node.json")
	if err != nil {
		t.Fatal(err)
	}
	changes := IntervalsFromEvents_NodeChanges(intervals, time.Time{}, time.Now())
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
