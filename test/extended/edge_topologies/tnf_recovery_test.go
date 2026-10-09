package edge_topologies

import (
	"reflect"
	"testing"

	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	corev1 "k8s.io/api/core/v1"
)

// TestUnexpectedFencingEventMessages verifies both independent old-event exclusions and both candidate nodes.
func TestUnexpectedFencingEventMessages(t *testing.T) {
	message := apis.FencingSuccessEventMessage("reboot", "master-1", "raw-completion")
	peerMessage := apis.FencingSuccessEventMessage("off", "master-0", "other-completion")
	failed := "Fencing event: off of master-1 completed with status failed (device timeout) at raw-completion"
	for _, test := range []struct {
		name   string
		before fencingSnapshot
		events []corev1.Event
		want   []string
	}{
		{name: "old Event without retained PCS history", before: fencingSnapshot{messages: map[string]bool{message: true}}, events: []corev1.Event{{Message: message}}},
		{name: "TTL recreated Event absent from message snapshot", before: fencingSnapshot{history: []services.FenceHistoryEvent{{Target: "master-1", Action: "off", Status: "failed", Completed: "raw-completion"}}}, events: []corev1.Event{{Message: message}}},
		{name: "old history also excludes failed reason message", before: fencingSnapshot{history: []services.FenceHistoryEvent{{Target: "master-1", Completed: "raw-completion"}}}, events: []corev1.Event{{Message: failed}}},
		{name: "new unknown fence Event is unexpected", events: []corev1.Event{{Message: message}}, want: []string{message}},
		{name: "both candidate nodes checked", events: []corev1.Event{{Message: message}, {Message: peerMessage}}, want: []string{message, peerMessage}},
		{name: "new failed Event is unexpected", events: []corev1.Event{{Type: corev1.EventTypeWarning, Message: failed}}, want: []string{failed}},
		{name: "same completion different target is unexpected", before: fencingSnapshot{history: []services.FenceHistoryEvent{{Target: "master-0", Completed: "raw-completion"}}}, events: []corev1.Event{{Message: message}}, want: []string{message}},
		{name: "same target different raw completion is unexpected", before: fencingSnapshot{history: []services.FenceHistoryEvent{{Target: "master-1", Completed: "raw-completion "}}}, events: []corev1.Event{{Message: message}}, want: []string{message}},
		{name: "noncandidate excluded", events: []corev1.Event{{Message: apis.FencingSuccessEventMessage("off", "master-10", "raw-completion")}}},
		{name: "unrelated excluded", events: []corev1.Event{{Message: "unrelated"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := unexpectedFencingEventMessages(test.events, test.before, "master-0", "master-1")
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
