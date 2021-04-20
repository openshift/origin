package monitor

import (
	"reflect"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/apimachinery/pkg/util/diff"
)

func TestMonitor_Newlines(t *testing.T) {
	evt := &monitorapi.EventInterval{Condition: monitorapi.Condition{Message: "a\nb\n"}}
	expected := "Jan 01 00:00:00.000 I  a\\nb\\n"
	if evt.String() != expected {
		t.Fatalf("unexpected:\n%s\n%s", expected, evt.String())
	}
}

func TestMonitor_Events(t *testing.T) {
	tests := []struct {
		name    string
		events  monitorapi.Intervals
		samples []*sample
		from    time.Time
		to      time.Time
		want    monitorapi.Intervals
	}{
		{
			events: monitorapi.Intervals{
				{Condition: monitorapi.Condition{Message: "1"}, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: monitorapi.Condition{Message: "2"}, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
			want: monitorapi.Intervals{
				{Condition: monitorapi.Condition{Message: "1"}, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: monitorapi.Condition{Message: "2"}, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
		},
		{
			events: monitorapi.Intervals{
				{Condition: monitorapi.Condition{Message: "1"}, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: monitorapi.Condition{Message: "2"}, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
			from: time.Unix(1, 0),
			want: monitorapi.Intervals{
				{Condition: monitorapi.Condition{Message: "2"}, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
		},
		{
			events: monitorapi.Intervals{
				{Condition: monitorapi.Condition{Message: "1"}, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: monitorapi.Condition{Message: "2"}, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
			from: time.Unix(1, 0),
			to:   time.Unix(2, 0),
			want: monitorapi.Intervals{
				{Condition: monitorapi.Condition{Message: "2"}, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
		},
		{
			events: monitorapi.Intervals{
				{Condition: monitorapi.Condition{Message: "1"}, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: monitorapi.Condition{Message: "2"}, From: time.Unix(2, 0), To: time.Unix(2, 0)},
			},
			from: time.Unix(2, 0),
			want: monitorapi.Intervals{},
		},
		{
			samples: []*sample{
				{at: time.Unix(1, 0), conditions: []*monitorapi.Condition{{Message: "1"}, {Message: "A"}}},
				{at: time.Unix(2, 0), conditions: []*monitorapi.Condition{{Message: "2"}}},
				{at: time.Unix(3, 0), conditions: []*monitorapi.Condition{{Message: "2"}, {Message: "A"}}},
			},
			from: time.Unix(1, 0),
			want: monitorapi.Intervals{
				{Condition: monitorapi.Condition{Message: "2"}, From: time.Unix(2, 0), To: time.Unix(3, 0)},
				{Condition: monitorapi.Condition{Message: "A"}, From: time.Unix(3, 0), To: time.Unix(3, 0)},
			},
		},
		{
			samples: []*sample{
				{at: time.Unix(1, 0), conditions: []*monitorapi.Condition{{Message: "1"}, {Message: "A"}}},
				{at: time.Unix(2, 0), conditions: []*monitorapi.Condition{{Message: "2"}}},
				{at: time.Unix(3, 0), conditions: []*monitorapi.Condition{{Message: "2"}, {Message: "A"}}},
			},
			want: monitorapi.Intervals{
				{Condition: monitorapi.Condition{Message: "1"}, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: monitorapi.Condition{Message: "A"}, From: time.Unix(1, 0), To: time.Unix(1, 0)},
				{Condition: monitorapi.Condition{Message: "2"}, From: time.Unix(2, 0), To: time.Unix(3, 0)},
				{Condition: monitorapi.Condition{Message: "A"}, From: time.Unix(3, 0), To: time.Unix(3, 0)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Monitor{
				events:  tt.events,
				samples: tt.samples,
			}
			if got := m.Intervals(tt.from, tt.to); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s", diff.ObjectReflectDiff(tt.want, got))
			}
		})
	}
}
