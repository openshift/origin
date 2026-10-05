package services

import (
	"encoding/xml"
	"reflect"
	"testing"
)

// pcsFenceHistoryFixture is synthetic full XML using fence attributes from a public
// CI log, not a verbatim captured runtime XML document.
const pcsFenceHistoryFixture = `<pacemaker-result api-version="2.33"><nodes><node name="master-1" online="true"/></nodes><fence_history status="0"><fence_event target="master-1" action="reboot" status="success" completed="2026-10-05 12:02:42.088147Z"/><fence_event target="master-1" action="reboot" status="success" completed="2026-10-05 11:54:33.972349Z"/></fence_history></pacemaker-result>`

// TestParsePcsFenceHistoryXML verifies the shared PCS response and optional retrieval status.
func TestParsePcsFenceHistoryXML(t *testing.T) {
	want := []FenceHistoryEvent{
		{Target: "master-1", Action: "reboot", Status: "success", Completed: "2026-10-05 12:02:42.088147Z"},
		{Target: "master-1", Action: "reboot", Status: "success", Completed: "2026-10-05 11:54:33.972349Z"},
	}
	var status pcsStatusXMLResponse
	if err := xml.Unmarshal([]byte(pcsFenceHistoryFixture), &status); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(status.FenceHistory.Events, want) || len(status.Nodes.Node) != 1 || status.Nodes.Node[0].Online != "true" {
		t.Fatalf("shared PCS response lost nodes or fence history: %#v", status)
	}
	for _, test := range []struct {
		name    string
		output  string
		want    []FenceHistoryEvent
		wantErr bool
	}{
		{name: "synthetic logged entries", output: pcsFenceHistoryFixture, want: want},
		{name: "missing status", output: `<pacemaker-result><fence_history/></pacemaker-result>`},
		{
			name:   "populated history without status",
			output: `<pacemaker-result><fence_history><fence_event target="master-1" action="reboot" status="success" completed="2026-10-05 12:02:42.088147Z"/></fence_history></pacemaker-result>`,
			want:   []FenceHistoryEvent{{Target: "master-1", Action: "reboot", Status: "success", Completed: "2026-10-05 12:02:42.088147Z"}},
		},
		{name: "missing history", output: `<pacemaker-result/>`},
		{name: "zero status", output: `<pacemaker-result><fence_history status="0"/></pacemaker-result>`},
		{name: "nonzero status", output: `<pacemaker-result><fence_history status="1"/></pacemaker-result>`, wantErr: true},
		{name: "malformed status", output: `<pacemaker-result><fence_history status="bad"/></pacemaker-result>`, wantErr: true},
		{name: "empty status", output: `<pacemaker-result><fence_history status=""/></pacemaker-result>`, wantErr: true},
		{name: "bad XML", output: `<pacemaker-result><fence_history>`, wantErr: true},
		{name: "bad root", output: `<other><fence_history/></other>`, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parsePcsFenceHistoryXML(test.output)
			if (err != nil) != test.wantErr || (!test.wantErr && !reflect.DeepEqual(got, test.want)) {
				t.Fatalf("got (%#v, %v), want (%#v, error=%v)", got, err, test.want, test.wantErr)
			}
		})
	}
}

// TestNewSuccessfulFences verifies target/action/completed snapshot identity and candidate success filtering.
func TestNewSuccessfulFences(t *testing.T) {
	old := FenceHistoryEvent{Target: "master-1", Action: "reboot", Status: "success", Completed: "2026-10-05 12:02:12.088147Z"}
	fresh := FenceHistoryEvent{Target: "master-1", Action: "reboot", Status: "success", Completed: "2026-10-05 12:02:42.088147Z"}
	peer := FenceHistoryEvent{Target: "master-0", Action: "off", Status: "success", Completed: "raw-completion"}
	failed := fresh
	failed.Status = "failed"
	tests := []struct {
		name   string
		before []FenceHistoryEvent
		after  []FenceHistoryEvent
		want   []FenceHistoryEvent
	}{
		{name: "retained old fence completed thirty seconds before disruption", before: []FenceHistoryEvent{old}, after: []FenceHistoryEvent{old}},
		{name: "new candidate success", before: []FenceHistoryEvent{old}, after: []FenceHistoryEvent{old, fresh}, want: []FenceHistoryEvent{fresh}},
		{name: "new fence on either candidate", after: []FenceHistoryEvent{fresh, peer}, want: []FenceHistoryEvent{fresh, peer}},
		{name: "duplicate whole entries", after: []FenceHistoryEvent{fresh, fresh, peer, peer}, want: []FenceHistoryEvent{fresh, peer}},
		{name: "status change for existing operation is not a new fence", before: []FenceHistoryEvent{failed}, after: []FenceHistoryEvent{fresh}},
		{name: "failed", after: []FenceHistoryEvent{{Target: "master-1", Action: "reboot", Status: "failed", Completed: "raw"}}},
		{name: "empty completion", after: []FenceHistoryEvent{{Target: "master-1", Action: "reboot", Status: "success"}}},
		{name: "noncandidate target", after: []FenceHistoryEvent{{Target: "master-10", Action: "reboot", Status: "success", Completed: "raw"}}},
		{name: "on is excluded", after: []FenceHistoryEvent{{Target: "master-1", Action: "on", Status: "success", Completed: "raw"}}},
		{name: "all prior entries are seen", before: []FenceHistoryEvent{old, peer, fresh}, after: []FenceHistoryEvent{fresh, peer, old}},
		{name: "empty snapshots"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := NewSuccessfulFences(test.before, test.after, "master-0", "master-1")
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
