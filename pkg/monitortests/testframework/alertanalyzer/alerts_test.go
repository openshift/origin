package alertanalyzer

import (
	"reflect"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

func Test_nonOverlappingBlackoutWindowsFromEvents(t *testing.T) {
	type args struct {
		blackoutWindows []monitorapi.Interval
	}
	tests := []struct {
		name string
		args args
		want []blackoutWindow
	}{
		{
			name: "no-overlap",
			args: args{
				blackoutWindows: []monitorapi.Interval{
					{
						From: timeOrDie("2022-03-22T19:00:00Z"),
						To:   timeOrDie("2022-03-22T19:10:00Z"),
					},
					{
						From: timeOrDie("2022-03-22T19:15:00Z"),
						To:   timeOrDie("2022-03-22T19:20:00Z"),
					},
				},
			},
			want: []blackoutWindow{
				{
					From: timeOrDie("2022-03-22T19:00:00Z"),
					To:   timeOrDie("2022-03-22T19:10:00Z"),
				},
				{
					From: timeOrDie("2022-03-22T19:15:00Z"),
					To:   timeOrDie("2022-03-22T19:20:00Z"),
				},
			},
		},
		{
			name: "fully-contained",
			args: args{
				blackoutWindows: []monitorapi.Interval{
					{
						From: timeOrDie("2022-03-22T19:00:00Z"),
						To:   timeOrDie("2022-03-22T19:10:00Z"),
					},
					{
						From: timeOrDie("2022-03-22T18:55:00Z"),
						To:   timeOrDie("2022-03-22T19:20:00Z"),
					},
				},
			},
			want: []blackoutWindow{
				{
					From: timeOrDie("2022-03-22T18:55:00Z"),
					To:   timeOrDie("2022-03-22T19:20:00Z"),
				},
			},
		},
		{
			name: "fully-contained-reverse",
			args: args{
				blackoutWindows: []monitorapi.Interval{
					{
						From: timeOrDie("2022-03-22T18:55:00Z"),
						To:   timeOrDie("2022-03-22T19:20:00Z"),
					},
					{
						From: timeOrDie("2022-03-22T19:00:00Z"),
						To:   timeOrDie("2022-03-22T19:10:00Z"),
					},
				},
			},
			want: []blackoutWindow{
				{
					From: timeOrDie("2022-03-22T18:55:00Z"),
					To:   timeOrDie("2022-03-22T19:20:00Z"),
				},
			},
		},
		{
			name: "overlap-beginning",
			args: args{
				blackoutWindows: []monitorapi.Interval{
					{
						From: timeOrDie("2022-03-22T19:00:00Z"),
						To:   timeOrDie("2022-03-22T19:10:00Z"),
					},
					{
						From: timeOrDie("2022-03-22T18:55:00Z"),
						To:   timeOrDie("2022-03-22T19:05:00Z"),
					},
				},
			},
			want: []blackoutWindow{
				{
					From: timeOrDie("2022-03-22T18:55:00Z"),
					To:   timeOrDie("2022-03-22T19:10:00Z"),
				},
			},
		},
		{
			name: "overlap-end",
			args: args{
				blackoutWindows: []monitorapi.Interval{
					{
						From: timeOrDie("2022-03-22T19:00:00Z"),
						To:   timeOrDie("2022-03-22T19:10:00Z"),
					},
					{
						From: timeOrDie("2022-03-22T19:05:00Z"),
						To:   timeOrDie("2022-03-22T19:20:00Z"),
					},
				},
			},
			want: []blackoutWindow{
				{
					From: timeOrDie("2022-03-22T19:00:00Z"),
					To:   timeOrDie("2022-03-22T19:20:00Z"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nonOverlappingBlackoutWindowsFromEvents(tt.args.blackoutWindows); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nonOverlappingBlackoutWindowsFromEvents() = %v, want %v", got, tt.want)
			}
		})
	}
}

func timeOrDie(in string) time.Time {
	startTime, err := time.Parse(time.RFC3339, in)
	if err != nil {
		panic(err)
	}
	return startTime
}

func Test_blackoutEvents(t *testing.T) {
	type args struct {
		startingEvents  []monitorapi.Interval
		blackoutWindows []monitorapi.Interval
	}
	conditionFoo := monitorapi.NewInterval(monitorapi.SourceAlert, monitorapi.Info).Locator(monitorapi.NewLocator().NodeFromName("foo")).BuildCondition()
	conditionBar := monitorapi.NewInterval(monitorapi.SourceAlert, monitorapi.Info).Locator(monitorapi.NewLocator().NodeFromName("bar")).BuildCondition()
	tests := []struct {
		name string
		args args
		want []monitorapi.Interval
	}{
		{
			name: "no-blackout",
			args: args{
				startingEvents: []monitorapi.Interval{
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:00:00Z"),
						To:        timeOrDie("2022-03-22T19:10:00Z"),
					},
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:05:00Z"),
						To:        timeOrDie("2022-03-22T19:20:00Z"),
					},
				},
				blackoutWindows: []monitorapi.Interval{
					{
						Condition: conditionBar,
						From:      timeOrDie("2022-03-22T19:00:00Z"),
						To:        timeOrDie("2022-03-22T19:10:00Z"),
					},
					{
						Condition: conditionBar,
						From:      timeOrDie("2022-03-22T19:05:00Z"),
						To:        timeOrDie("2022-03-22T19:20:00Z"),
					},
				},
			},
			want: []monitorapi.Interval{
				{
					Condition: conditionFoo,
					From:      timeOrDie("2022-03-22T19:00:00Z"),
					To:        timeOrDie("2022-03-22T19:10:00Z"),
				},
				{
					Condition: conditionFoo,
					From:      timeOrDie("2022-03-22T19:05:00Z"),
					To:        timeOrDie("2022-03-22T19:20:00Z"),
				},
			},
		},
		{
			name: "full-blackout",
			args: args{
				startingEvents: []monitorapi.Interval{
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:00:00Z"),
						To:        timeOrDie("2022-03-22T19:10:00Z"),
					},
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:05:00Z"),
						To:        timeOrDie("2022-03-22T19:20:00Z"),
					},
				},
				blackoutWindows: []monitorapi.Interval{
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:00:00Z"),
						To:        timeOrDie("2022-03-22T19:08:00Z"),
					},
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:05:00Z"),
						To:        timeOrDie("2022-03-22T19:20:00Z"),
					},
				},
			},
			want: []monitorapi.Interval{},
		},
		{
			name: "full-and-trailing-section-blackout",
			args: args{
				startingEvents: []monitorapi.Interval{
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:00:00Z"),
						To:        timeOrDie("2022-03-22T19:10:00Z"),
					},
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:05:00Z"),
						To:        timeOrDie("2022-03-22T19:20:00Z"),
					},
				},
				blackoutWindows: []monitorapi.Interval{
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:05:00Z"),
						To:        timeOrDie("2022-03-22T19:20:00Z"),
					},
				},
			},
			want: []monitorapi.Interval{
				{
					Condition: conditionFoo,
					From:      timeOrDie("2022-03-22T19:00:00Z"),
					To:        timeOrDie("2022-03-22T19:05:00Z"),
				},
			},
		},
		{
			name: "partial-blackouts",
			args: args{
				startingEvents: []monitorapi.Interval{
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:00:00Z"),
						To:        timeOrDie("2022-03-22T19:10:00Z"),
					},
				},
				blackoutWindows: []monitorapi.Interval{
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:01:00Z"),
						To:        timeOrDie("2022-03-22T19:02:00Z"),
					},
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:04:00Z"),
						To:        timeOrDie("2022-03-22T19:05:00Z"),
					},
				},
			},
			want: []monitorapi.Interval{
				{
					Condition: conditionFoo,
					From:      timeOrDie("2022-03-22T19:00:00Z"),
					To:        timeOrDie("2022-03-22T19:01:00Z"),
				},
				{
					Condition: conditionFoo,
					From:      timeOrDie("2022-03-22T19:02:00Z"),
					To:        timeOrDie("2022-03-22T19:04:00Z"),
				},
				{
					Condition: conditionFoo,
					From:      timeOrDie("2022-03-22T19:05:00Z"),
					To:        timeOrDie("2022-03-22T19:10:00Z"),
				},
			},
		},
		{
			name: "leading-blackouts",
			args: args{
				startingEvents: []monitorapi.Interval{
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:00:00Z"),
						To:        timeOrDie("2022-03-22T19:10:00Z"),
					},
				},
				blackoutWindows: []monitorapi.Interval{
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T18:55:00Z"),
						To:        timeOrDie("2022-03-22T19:02:00Z"),
					},
					{
						Condition: conditionFoo,
						From:      timeOrDie("2022-03-22T19:04:00Z"),
						To:        timeOrDie("2022-03-22T19:05:00Z"),
					},
				},
			},
			want: []monitorapi.Interval{
				{
					Condition: conditionFoo,
					From:      timeOrDie("2022-03-22T19:02:00Z"),
					To:        timeOrDie("2022-03-22T19:04:00Z"),
				},
				{
					Condition: conditionFoo,
					From:      timeOrDie("2022-03-22T19:05:00Z"),
					To:        timeOrDie("2022-03-22T19:10:00Z"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blackoutEvents(tt.args.startingEvents, tt.args.blackoutWindows)
			assert.Equal(t, tt.want, got)
		})
	}
}
