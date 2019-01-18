package monitor

import (
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/diff"
)

func TestMonitor_Newlines(t *testing.T) {
	evt := &Event{Condition: Condition{Message: "a\nb\n"}}
	expected := "Jan 01 00:00:00.000 I  a\\nb\\n"
	if evt.String() != expected {
		t.Fatalf("unexpected:\n%s\n%s", expected, evt.String())
	}
}

func TestMonitor_Events(t *testing.T) {
	tests := []struct {
		name    string
		events  []*Event
		samples []*sample
		from    time.Time
		to      time.Time
		want    EventIntervals
	}{
		{
			events: []*Event{
				{Condition{Message: "1"}, time.Unix(1, 0)},
				{Condition{Message: "2"}, time.Unix(2, 0)},
			},
			want: EventIntervals{
				{&Condition{Message: "1"}, time.Unix(1, 0), time.Unix(1, 0)},
				{&Condition{Message: "2"}, time.Unix(2, 0), time.Unix(2, 0)},
			},
		},
		{
			events: []*Event{
				{Condition{Message: "1"}, time.Unix(1, 0)},
				{Condition{Message: "2"}, time.Unix(2, 0)},
			},
			from: time.Unix(1, 0),
			want: EventIntervals{
				{&Condition{Message: "2"}, time.Unix(2, 0), time.Unix(2, 0)},
			},
		},
		{
			events: []*Event{
				{Condition{Message: "1"}, time.Unix(1, 0)},
				{Condition{Message: "2"}, time.Unix(2, 0)},
			},
			from: time.Unix(1, 0),
			to:   time.Unix(2, 0),
			want: EventIntervals{
				{&Condition{Message: "2"}, time.Unix(2, 0), time.Unix(2, 0)},
			},
		},
		{
			events: []*Event{
				{Condition{Message: "1"}, time.Unix(1, 0)},
				{Condition{Message: "2"}, time.Unix(2, 0)},
			},
			from: time.Unix(2, 0),
			want: nil,
		},
		{
			samples: []*sample{
				{time.Unix(1, 0), []*Condition{{Message: "1"}, {Message: "A"}}},
				{time.Unix(2, 0), []*Condition{{Message: "2"}}},
				{time.Unix(3, 0), []*Condition{{Message: "2"}, {Message: "A"}}},
			},
			from: time.Unix(1, 0),
			want: EventIntervals{
				{&Condition{Message: "2"}, time.Unix(2, 0), time.Unix(3, 0)},
				{&Condition{Message: "A"}, time.Unix(3, 0), time.Unix(3, 0)},
			},
		},
		{
			samples: []*sample{
				{time.Unix(1, 0), []*Condition{{Message: "1"}, {Message: "A"}}},
				{time.Unix(2, 0), []*Condition{{Message: "2"}}},
				{time.Unix(3, 0), []*Condition{{Message: "2"}, {Message: "A"}}},
			},
			want: EventIntervals{
				{&Condition{Message: "1"}, time.Unix(1, 0), time.Unix(1, 0)},
				{&Condition{Message: "A"}, time.Unix(1, 0), time.Unix(1, 0)},
				{&Condition{Message: "2"}, time.Unix(2, 0), time.Unix(3, 0)},
				{&Condition{Message: "A"}, time.Unix(3, 0), time.Unix(3, 0)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Monitor{
				events:  tt.events,
				samples: tt.samples,
			}
			if got := m.Events(tt.from, tt.to); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s", diff.ObjectReflectDiff(tt.want, got))
			}
		})
	}
}
