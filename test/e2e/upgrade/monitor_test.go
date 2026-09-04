package upgrade

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
)

func TestCVOAcknowledgedUpdate(t *testing.T) {
	desired := configv1.Update{Version: "5.0.0", Image: "quay.io/openshift-release-dev/ocp-release@sha256:abc", Force: true}
	targetMessage := "Retrieving and verifying payload version=\"5.0.0\" image=\"quay.io/openshift-release-dev/ocp-release@sha256:abc\""
	matchingEvent := v1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "version.abc", ResourceVersion: "2"},
		InvolvedObject: v1.ObjectReference{
			APIVersion: configv1.GroupVersion.String(),
			Kind:       clusterVersionKind,
			Name:       "version",
		},
		Reason:  retrievePayloadReason,
		Message: targetMessage,
		Source:  v1.EventSource{Component: cvoNamespace},
		Type:    v1.EventTypeNormal,
	}
	matchingCondition := configv1.ClusterOperatorStatusCondition{
		Type:    releaseAcceptedConditionType,
		Status:  configv1.ConditionUnknown,
		Reason:  retrievePayloadReason,
		Message: targetMessage,
	}
	versionOnlyDesired := configv1.Update{Version: "5.0.0", Force: true}
	versionOnlyMessage := "Retrieving and verifying payload version=\"5.0.0\" image=\"resolved-image\""
	versionOnlyEvent := matchingEvent
	versionOnlyEvent.Name = "version.def"
	versionOnlyEvent.Message = versionOnlyMessage
	versionOnlyCondition := matchingCondition
	versionOnlyCondition.Message = versionOnlyMessage

	tests := []struct {
		name     string
		desired  *configv1.Update
		cv       *configv1.ClusterVersion
		events   []v1.Event
		baseline cvoAcknowledgementBaseline
		want     bool
		wantErr  bool
	}{
		{
			name:     "version-only update from retrieve payload event",
			desired:  &versionOnlyDesired,
			cv:       clusterVersionForAcknowledgement(versionOnlyDesired, 1, nil),
			events:   []v1.Event{versionOnlyEvent},
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}, eventsAvailable: true},
			want:     true,
		},
		{
			name:     "version-only update from retrieve payload condition",
			desired:  &versionOnlyDesired,
			cv:       clusterVersionForAcknowledgement(versionOnlyDesired, 1, []configv1.ClusterOperatorStatusCondition{versionOnlyCondition}),
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}},
			want:     true,
		},
		{
			name:    "version-only update from event for another version",
			desired: &versionOnlyDesired,
			cv:      clusterVersionForAcknowledgement(versionOnlyDesired, 1, nil),
			events: []v1.Event{func() v1.Event {
				event := versionOnlyEvent
				event.Message = "Retrieving and verifying payload version=\"5.0.1\" image=\"resolved-image\""
				return event
			}()},
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}, eventsAvailable: true},
		},
		{
			name:    "version-only update from condition for another version",
			desired: &versionOnlyDesired,
			cv: clusterVersionForAcknowledgement(versionOnlyDesired, 1, []configv1.ClusterOperatorStatusCondition{func() configv1.ClusterOperatorStatusCondition {
				condition := versionOnlyCondition
				condition.Message = "Retrieving and verifying payload version=\"5.0.1\" image=\"resolved-image\""
				return condition
			}()}),
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}},
		},
		{
			name: "image-specific update from event with another image",
			cv:   clusterVersionForAcknowledgement(desired, 1, nil),
			events: []v1.Event{func() v1.Event {
				event := matchingEvent
				event.Message = "Retrieving and verifying payload version=\"5.0.0\" image=\"other-image\""
				return event
			}()},
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}, eventsAvailable: true},
		},
		{
			name: "image-specific update from condition with another image",
			cv: clusterVersionForAcknowledgement(desired, 1, []configv1.ClusterOperatorStatusCondition{func() configv1.ClusterOperatorStatusCondition {
				condition := matchingCondition
				condition.Message = "Retrieving and verifying payload version=\"5.0.0\" image=\"other-image\""
				return condition
			}()}),
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}},
		},
		{
			name: "observed generation",
			cv:   clusterVersionForAcknowledgement(desired, 2, nil),
			want: true,
		},
		{
			name:     "new retrieve payload event",
			cv:       clusterVersionForAcknowledgement(desired, 1, nil),
			events:   []v1.Event{matchingEvent},
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}, eventsAvailable: true},
			want:     true,
		},
		{
			name:     "updated retrieve payload event",
			cv:       clusterVersionForAcknowledgement(desired, 1, nil),
			events:   []v1.Event{matchingEvent},
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{matchingEvent.Name: "1"}, eventsAvailable: true},
			want:     true,
		},
		{
			name:     "stale retrieve payload event",
			cv:       clusterVersionForAcknowledgement(desired, 1, nil),
			events:   []v1.Event{matchingEvent},
			baseline: newCVOAcknowledgementBaseline(clusterVersionForAcknowledgement(desired, 1, nil), []v1.Event{matchingEvent}, true),
		},
		{
			name: "event for another target",
			cv:   clusterVersionForAcknowledgement(desired, 1, nil),
			events: []v1.Event{func() v1.Event {
				event := matchingEvent
				event.Message = "Retrieving and verifying payload version=\"5.0.1\" image=\"other-image\""
				return event
			}()},
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}, eventsAvailable: true},
		},
		{
			name: "event for another object",
			cv:   clusterVersionForAcknowledgement(desired, 1, nil),
			events: []v1.Event{func() v1.Event {
				event := matchingEvent
				event.InvolvedObject.Name = "other"
				return event
			}()},
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}, eventsAvailable: true},
		},
		{
			name:     "event baseline unavailable",
			cv:       clusterVersionForAcknowledgement(desired, 1, nil),
			events:   []v1.Event{matchingEvent},
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}},
		},
		{
			name: "event from another source",
			cv:   clusterVersionForAcknowledgement(desired, 1, nil),
			events: []v1.Event{func() v1.Event {
				event := matchingEvent
				event.Source.Component = "other"
				return event
			}()},
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}, eventsAvailable: true},
		},
		{
			name:     "new retrieve payload condition",
			cv:       clusterVersionForAcknowledgement(desired, 1, []configv1.ClusterOperatorStatusCondition{matchingCondition}),
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}},
			want:     true,
		},
		{
			name:     "stale retrieve payload condition",
			cv:       clusterVersionForAcknowledgement(desired, 1, []configv1.ClusterOperatorStatusCondition{matchingCondition}),
			baseline: cvoAcknowledgementBaseline{releaseAccepted: &matchingCondition, eventResourceVersion: map[string]string{}},
		},
		{
			name: "failed retrieve payload condition",
			cv: clusterVersionForAcknowledgement(desired, 1, []configv1.ClusterOperatorStatusCondition{{
				Type: releaseAcceptedConditionType, Status: configv1.ConditionFalse, Reason: retrievePayloadReason,
				Message: "Retrieving payload failed version=\"5.0.0\" image=\"quay.io/openshift-release-dev/ocp-release@sha256:abc\" failure=timed out",
			}}),
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}},
		},
		{
			name: "condition for another target",
			cv: clusterVersionForAcknowledgement(desired, 1, []configv1.ClusterOperatorStatusCondition{{
				Type: releaseAcceptedConditionType, Status: configv1.ConditionUnknown, Reason: retrievePayloadReason,
				Message: "Retrieving and verifying payload version=\"5.0.1\" image=\"other-image\"",
			}}),
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}},
		},
		{
			name: "unrelated release accepted condition",
			cv: clusterVersionForAcknowledgement(desired, 1, []configv1.ClusterOperatorStatusCondition{{
				Type: releaseAcceptedConditionType, Status: configv1.ConditionTrue, Reason: "PayloadLoaded", Message: targetMessage,
			}}),
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}},
		},
		{
			name:     "desired update changed with acknowledgement signal",
			cv:       clusterVersionForAcknowledgement(configv1.Update{Version: "5.0.1", Image: "other-image", Force: true}, 2, nil),
			events:   []v1.Event{matchingEvent},
			baseline: cvoAcknowledgementBaseline{eventResourceVersion: map[string]string{}},
			wantErr:  true,
		},
		{
			name:    "desired update cleared",
			cv:      &configv1.ClusterVersion{ObjectMeta: metav1.ObjectMeta{Name: "version"}},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testDesired := desired
			if test.desired != nil {
				testDesired = *test.desired
			}
			got, err := cvoAcknowledgedUpdate(test.cv, 2, testDesired, test.events, test.baseline)
			if (err != nil) != test.wantErr {
				t.Fatalf("cvoAcknowledgedUpdate() error = %v, wantErr %t", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("cvoAcknowledgedUpdate() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestNewCVOAcknowledgementBaseline(t *testing.T) {
	condition := configv1.ClusterOperatorStatusCondition{
		Type: releaseAcceptedConditionType, Status: configv1.ConditionTrue, Reason: "PayloadLoaded",
	}
	events := []v1.Event{{ObjectMeta: metav1.ObjectMeta{Name: "event", ResourceVersion: "1"}}}
	cv := clusterVersionForAcknowledgement(configv1.Update{Image: "image"}, 1, []configv1.ClusterOperatorStatusCondition{condition})

	baseline := newCVOAcknowledgementBaseline(cv, events, true)
	cv.Status.Conditions[0].Reason = "changed"
	events[0].ResourceVersion = "2"

	if baseline.releaseAccepted == nil || !reflect.DeepEqual(*baseline.releaseAccepted, condition) {
		t.Errorf("releaseAccepted = %#v, want independent copy %#v", baseline.releaseAccepted, condition)
	}
	if got := baseline.eventResourceVersion["event"]; got != "1" {
		t.Errorf("event resource version = %q, want %q", got, "1")
	}
	if !baseline.eventsAvailable {
		t.Error("eventsAvailable = false, want true")
	}
}

func clusterVersionForAcknowledgement(desired configv1.Update, observedGeneration int64, conditions []configv1.ClusterOperatorStatusCondition) *configv1.ClusterVersion {
	return &configv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{Name: "version"},
		Spec:       configv1.ClusterVersionSpec{DesiredUpdate: &desired},
		Status: configv1.ClusterVersionStatus{
			ObservedGeneration: observedGeneration,
			Conditions:         conditions,
		},
	}
}
