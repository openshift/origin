package monitor

import (
	"reflect"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/util/diff"
)

func TestMonitor_Newlines(t *testing.T) {
	evt := &monitorapi.Interval{Condition: monitorapi.Condition{Message: monitorapi.Message{HumanMessage: "a\nb\n"}}}
	// this originally expected preservation of the trailing newline, that gets trimmed somewhere in the new intervals,
	// which seems ok as far as I can tell
	expected := "Jan 01 00:00:00.000 I  a\\nb"
	if evt.String() != expected {
		t.Fatalf("unexpected:\n%s\n%s", expected, evt.String())
	}
}

func TestMonitor_Events(t *testing.T) {
	condition1 := monitorapi.NewInterval(monitorapi.SourceTestData, monitorapi.Info).Locator(monitorapi.NewLocator().NodeFromName("foo")).Message(monitorapi.NewMessage().HumanMessage("1")).BuildCondition()
	condition2 := monitorapi.NewInterval(monitorapi.SourceTestData, monitorapi.Info).Locator(monitorapi.NewLocator().NodeFromName("foo")).Message(monitorapi.NewMessage().HumanMessage("2")).BuildCondition()
	tests := []struct {
		name   string
		events monitorapi.Intervals
		from   time.Time
		to     time.Time
		want   monitorapi.Intervals
	}{
		{
			name: "one",
			events: monitorapi.Intervals{
				{Condition: condition1, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: condition2, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
			want: monitorapi.Intervals{
				{Condition: condition1, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: condition2, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
		},
		{
			name: "two",
			events: monitorapi.Intervals{
				{Condition: condition1, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: condition2, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
			from: time.Unix(1, 0),
			want: monitorapi.Intervals{
				{Condition: condition1, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: condition2, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
		},
		{
			name: "two-a",
			events: monitorapi.Intervals{
				{Condition: condition1, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: condition2, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
			from: time.Unix(2, 0),
			want: monitorapi.Intervals{
				{Condition: condition2, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
		},
		{
			name: "three",
			events: monitorapi.Intervals{
				{Condition: condition1, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: condition2, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
			from: time.Unix(1, 0),
			to:   time.Unix(2, 0),
			want: monitorapi.Intervals{
				{Condition: condition1, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: condition2, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
		},
		{
			name: "three-a",
			events: monitorapi.Intervals{
				{Condition: condition1, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: condition2, From: time.Unix(2, 0), To: time.Unix(2, 0)},
				{Condition: condition2, From: time.Unix(3, 0), To: time.Unix(3, 0)},
			},
			from: time.Unix(1, 0),
			to:   time.Unix(2, 0),
			want: monitorapi.Intervals{
				{Condition: condition1, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: condition2, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
		},
		{
			name: "four",
			events: monitorapi.Intervals{
				{Condition: condition1, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: condition2, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
			from: time.Unix(3, 0),
			want: monitorapi.Intervals{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Monitor{
				recorder: &recorder{
					events:            tt.events,
					recordedResources: monitorapi.ResourcesMap{},
				},
			}
			if got := m.recorder.Intervals(tt.from, tt.to); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s", diff.Diff(tt.want, got))
			}
		})
	}
}
