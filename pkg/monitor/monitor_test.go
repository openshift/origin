package monitor

import (
	"reflect"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/apimachinery/pkg/util/diff"
)

func TestMonitor_Newlines(t *testing.T) {
	evt := &monitorapi.Event{Condition: monitorapi.Condition{Message: "a\nb\n"}}
	expected := "Jan 01 00:00:00.000 I  a\\nb\\n"
	if evt.String() != expected {
		t.Fatalf("unexpected:\n%s\n%s", expected, evt.String())
	}
}

func TestMonitor_Events(t *testing.T) {
	tests := []struct {
		name    string
		events  []*monitorapi.Event
		samples []*sample
		from    time.Time
		to      time.Time
		want    monitorapi.EventIntervals
	}{
		{
			events: []*monitorapi.Event{
				{monitorapi.Condition{Message: "1"}, time.Unix(1, 0)},
				{monitorapi.Condition{Message: "2"}, time.Unix(2, 0)},
			},
			want: monitorapi.EventIntervals{
				{&monitorapi.Condition{Message: "1"}, time.Unix(1, 0), time.Unix(1, 0)},
				{&monitorapi.Condition{Message: "2"}, time.Unix(2, 0), time.Unix(2, 0)},
			},
		},
		{
			events: []*monitorapi.Event{
				{monitorapi.Condition{Message: "1"}, time.Unix(1, 0)},
				{monitorapi.Condition{Message: "2"}, time.Unix(2, 0)},
			},
			from: time.Unix(1, 0),
			want: monitorapi.EventIntervals{
				{&monitorapi.Condition{Message: "2"}, time.Unix(2, 0), time.Unix(2, 0)},
			},
		},
		{
			events: []*monitorapi.Event{
				{monitorapi.Condition{Message: "1"}, time.Unix(1, 0)},
				{monitorapi.Condition{Message: "2"}, time.Unix(2, 0)},
			},
			from: time.Unix(1, 0),
			to:   time.Unix(2, 0),
			want: monitorapi.EventIntervals{
				{&monitorapi.Condition{Message: "2"}, time.Unix(2, 0), time.Unix(2, 0)},
			},
		},
		{
			events: []*monitorapi.Event{
				{monitorapi.Condition{Message: "1"}, time.Unix(1, 0)},
				{monitorapi.Condition{Message: "2"}, time.Unix(2, 0)},
			},
			from: time.Unix(2, 0),
			want: nil,
		},
		{
			samples: []*sample{
				{time.Unix(1, 0), []*monitorapi.Condition{{Message: "1"}, {Message: "A"}}},
				{time.Unix(2, 0), []*monitorapi.Condition{{Message: "2"}}},
				{time.Unix(3, 0), []*monitorapi.Condition{{Message: "2"}, {Message: "A"}}},
			},
			from: time.Unix(1, 0),
			want: monitorapi.EventIntervals{
				{&monitorapi.Condition{Message: "2"}, time.Unix(2, 0), time.Unix(3, 0)},
				{&monitorapi.Condition{Message: "A"}, time.Unix(3, 0), time.Unix(3, 0)},
			},
		},
		{
			samples: []*sample{
				{time.Unix(1, 0), []*monitorapi.Condition{{Message: "1"}, {Message: "A"}}},
				{time.Unix(2, 0), []*monitorapi.Condition{{Message: "2"}}},
				{time.Unix(3, 0), []*monitorapi.Condition{{Message: "2"}, {Message: "A"}}},
			},
			want: monitorapi.EventIntervals{
				{&monitorapi.Condition{Message: "1"}, time.Unix(1, 0), time.Unix(1, 0)},
				{&monitorapi.Condition{Message: "A"}, time.Unix(1, 0), time.Unix(1, 0)},
				{&monitorapi.Condition{Message: "2"}, time.Unix(2, 0), time.Unix(3, 0)},
				{&monitorapi.Condition{Message: "A"}, time.Unix(3, 0), time.Unix(3, 0)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Monitor{
				events:  tt.events,
				samples: tt.samples,
			}
			if got := m.EventIntervals(tt.from, tt.to); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s", diff.ObjectReflectDiff(tt.want, got))
			}
		})
	}
}
