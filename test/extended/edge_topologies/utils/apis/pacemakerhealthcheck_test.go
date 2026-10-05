package apis

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// TestFencingSuccessEventMessage verifies the exact CEO success format and raw completion preservation.
func TestFencingSuccessEventMessage(t *testing.T) {
	for _, action := range []string{"reboot", "off"} {
		t.Run(action, func(t *testing.T) {
			got := FencingSuccessEventMessage(action, "master-1", "2026-10-05 12:02:42.088147Z")
			want := "Fencing event: " + action + " of master-1 completed with status success at 2026-10-05 12:02:42.088147Z"
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

// TestParseFencingEvent verifies anchored messages and opaque target/completion extraction.
func TestParseFencingEvent(t *testing.T) {
	for _, test := range []struct {
		name, message, target, completed string
		ok                               bool
	}{
		{name: "success", message: "Fencing event: reboot of master-1 completed with status success at 2026-10-05 12:02:42.088147Z", target: "master-1", completed: "2026-10-05 12:02:42.088147Z", ok: true},
		{name: "failed with reason", message: "Fencing event: off of master-0 completed with status failed (device rejected (timeout)) at raw-completion", target: "master-0", completed: "raw-completion", ok: true},
		{name: "status is not authority", message: "Fencing event: off of master-10 completed with status success-extra at not-a-time", target: "master-10", completed: "not-a-time", ok: true},
		{name: "prefix rejected", message: "collector: Fencing event: off of master-1 completed with status success at raw"},
		{name: "on rejected", message: "Fencing event: on of master-1 completed with status success at raw"},
		{name: "empty completion rejected", message: "Fencing event: off of master-1 completed with status success at "},
		{name: "unrelated", message: "master-1 completed with status success"},
	} {
		t.Run(test.name, func(t *testing.T) {
			target, completed, ok := ParseFencingEvent(&corev1.Event{Message: test.message})
			if target != test.target || completed != test.completed || ok != test.ok {
				t.Fatalf("got (%q, %q, %v), want (%q, %q, %v)", target, completed, ok, test.target, test.completed, test.ok)
			}
		})
	}
}

// TestFencingEventsWithMessage verifies exact equality independent of Type, Count, and other messages.
func TestFencingEventsWithMessage(t *testing.T) {
	message := FencingSuccessEventMessage("reboot", "master-1", "2026-10-05 12:02:42.088147Z")
	normal := corev1.Event{Type: corev1.EventTypeNormal, Message: message, Count: 999}
	warning := corev1.Event{Type: corev1.EventTypeWarning, Message: message}
	for _, test := range []struct {
		name   string
		events []corev1.Event
		want   []corev1.Event
	}{
		{name: "exact Normal success", events: []corev1.Event{normal}, want: []corev1.Event{normal}},
		{name: "only message is filtered", events: []corev1.Event{normal, warning}, want: []corev1.Event{normal, warning}},
		{name: "prefix near miss", events: []corev1.Event{{Message: "collector: " + message}}},
		{name: "suffix near miss", events: []corev1.Event{{Message: message + " "}}},
		{name: "different target", events: []corev1.Event{{Message: FencingSuccessEventMessage("reboot", "master-10", "2026-10-05 12:02:42.088147Z")}}},
		{name: "different raw completion", events: []corev1.Event{{Message: FencingSuccessEventMessage("reboot", "master-1", "2026-10-05 12:02:42.088147+00:00")}}},
		{name: "failed attempt ignored", events: []corev1.Event{{Message: "Fencing event: reboot of master-1 completed with status failed (completed with status success) at 2026-10-05 12:02:42.088147Z"}, normal}, want: []corev1.Event{normal}},
		{name: "two exact objects remain", events: []corev1.Event{normal, normal}, want: []corev1.Event{normal, normal}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := FencingEventsWithMessage(test.events, message)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
}
