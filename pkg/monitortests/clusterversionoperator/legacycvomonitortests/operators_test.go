package legacycvomonitortests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

// buildUpgradeInterval creates a standard upgrade interval using the given reason and eventTime.
// These are the fields used by the isInUpgradeWindow function.
func buildUpgradeInterval(reason monitorapi.IntervalReason, eventTime time.Time) monitorapi.Interval {
	interval := monitorapi.Interval{
		Condition: monitorapi.Condition{
			Locator: monitorapi.Locator{
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorClusterVersionKey: "cluster",
				},
			},
			Message: monitorapi.Message{
				Reason: reason,
			},
		},
		Source: monitorapi.SourceKubeEvent,
		From:   eventTime,
		To:     eventTime,
	}
	return interval
}

type upgradeEvent struct {
	eventTime time.Time
	reason    monitorapi.IntervalReason
}

// intervalWithSingleTime creates a monitorapi.Interval with the same start and end time.
func intervalWithSingleTime(eventTime time.Time) monitorapi.Interval {
	return monitorapi.Interval{
		From: eventTime,
		To:   eventTime,
	}
}

// makeUpgradeEventList creates a list of standard upgrade events using
// output from parsing the e2e-events.file
//
//	cat e2e-events_20240502-205107.json | jq '.items[] | \
//	   select(.source == "KubeEvent" and .locator.keys.clusterversion? == "cluster")| \
//	   "\(.from) \(.to) \(.message.reason)"'
func makeUpgradeEventList(events []upgradeEvent) monitorapi.Intervals {
	var intervals monitorapi.Intervals
	for _, event := range events {
		interval := buildUpgradeInterval(event.reason, event.eventTime)
		intervals = append(intervals, interval)
	}
	return intervals
}

func Test_isInUpgradeWindow(t *testing.T) {
	type args struct {
		eventList     monitorapi.Intervals
		eventInterval monitorapi.Interval
	}

	standardEventList := makeUpgradeEventList([]upgradeEvent{
		{eventTime: time.Date(2024, 5, 1, 12, 51, 9, 0, time.UTC), reason: monitorapi.UpgradeStartedReason},
		{eventTime: time.Date(2024, 5, 1, 13, 46, 44, 0, time.UTC), reason: monitorapi.UpgradeVersionReason},
		{eventTime: time.Date(2024, 5, 1, 13, 46, 44, 0, time.UTC), reason: monitorapi.UpgradeCompleteReason},
	})

	eventListWithRollback := makeUpgradeEventList([]upgradeEvent{
		{eventTime: time.Date(2024, 5, 1, 22, 21, 42, 0, time.UTC), reason: monitorapi.UpgradeStartedReason},
		{eventTime: time.Date(2024, 5, 1, 23, 15, 8, 0, time.UTC), reason: monitorapi.UpgradeRollbackReason},
		{eventTime: time.Date(2024, 5, 2, 0, 11, 18, 0, time.UTC), reason: monitorapi.UpgradeVersionReason},
		{eventTime: time.Date(2024, 5, 2, 0, 11, 18, 0, time.UTC), reason: monitorapi.UpgradeCompleteReason},
	})

	rollbackHappenedBeforeStartList := makeUpgradeEventList([]upgradeEvent{
		{eventTime: time.Date(2024, 5, 1, 13, 46, 44, 0, time.UTC), reason: monitorapi.UpgradeCompleteReason},
	})

	rollbackAfterErroneousCompletionList := makeUpgradeEventList([]upgradeEvent{
		{eventTime: time.Date(2024, 5, 2, 11, 0, 0, 0, time.UTC), reason: monitorapi.UpgradeCompleteReason},
		{eventTime: time.Date(2024, 5, 2, 11, 45, 0, 0, time.UTC), reason: monitorapi.UpgradeRollbackReason},
	})

	rollbackAfterCompletionList := makeUpgradeEventList([]upgradeEvent{
		{eventTime: time.Date(2024, 5, 2, 11, 0, 0, 0, time.UTC), reason: monitorapi.UpgradeStartedReason},
		{eventTime: time.Date(2024, 5, 2, 11, 30, 0, 0, time.UTC), reason: monitorapi.UpgradeCompleteReason},
		{eventTime: time.Date(2024, 5, 2, 11, 45, 0, 0, time.UTC), reason: monitorapi.UpgradeRollbackReason},
	})
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "A rollback followed an upgrade completion",
			args: args{
				eventList:     rollbackAfterCompletionList,
				eventInterval: intervalWithSingleTime(time.Date(2024, 5, 2, 11, 35, 0, 0, time.UTC)),
			},
			want: true,
		},
		{
			name: "A rollback followed an erroneous upgrade completion (error condition)",
			args: args{
				eventList:     rollbackAfterErroneousCompletionList,
				eventInterval: intervalWithSingleTime(time.Date(2024, 5, 2, 11, 30, 0, 0, time.UTC)),
			},
			want: false,
		},
		{
			name: "An upgrade completion happened before an upgrade start or rollback (error condition)",
			args: args{
				eventList:     rollbackHappenedBeforeStartList,
				eventInterval: intervalWithSingleTime(time.Date(2024, 5, 2, 11, 20, 0, 0, time.UTC)),
			},
			want: false,
		},
		{
			name: "single upgrade window, interval not within",
			args: args{
				eventList:     standardEventList,
				eventInterval: intervalWithSingleTime(time.Date(2024, 5, 1, 12, 49, 28, 0, time.UTC)),
			},
			want: false,
		},
		{
			name: "single upgrade window, interval within",
			args: args{
				eventList:     standardEventList,
				eventInterval: intervalWithSingleTime(time.Date(2024, 5, 1, 13, 44, 28, 0, time.UTC)),
			},
			want: true,
		},
		{
			name: "single upgrade window, with no end",
			args: args{
				eventList:     standardEventList[0:2],
				eventInterval: intervalWithSingleTime(time.Date(2024, 5, 1, 14, 0, 0, 0, time.UTC)),
			},
			want: true,
		},
		{
			name: "upgrade with rollback, interval before first upgrade",
			args: args{
				eventList:     eventListWithRollback,
				eventInterval: intervalWithSingleTime(time.Date(2024, 5, 1, 22, 10, 0, 0, time.UTC)),
			},
			want: false,
		},
		{
			name: "upgrade with rollback, interval inside first upgrade",
			args: args{
				eventList:     eventListWithRollback,
				eventInterval: intervalWithSingleTime(time.Date(2024, 5, 1, 22, 25, 0, 0, time.UTC)),
			},
			want: true,
		},
		{
			name: "upgrade with rollback, interval inside rollback",
			args: args{
				eventList:     eventListWithRollback,
				eventInterval: intervalWithSingleTime(time.Date(2024, 5, 1, 23, 18, 0, 0, time.UTC)),
			},
			want: true,
		},
		{
			name: "upgrade with rollback, interval past end of rollback",
			args: args{
				eventList:     eventListWithRollback,
				eventInterval: intervalWithSingleTime(time.Date(2024, 5, 2, 11, 20, 0, 0, time.UTC)),
			},
			want: false,
		},
		{
			name: "upgrade with rollback, interval past end of rollback with no end",
			args: args{
				eventList:     eventListWithRollback[0:3],
				eventInterval: intervalWithSingleTime(time.Date(2024, 5, 2, 11, 20, 0, 0, time.UTC)),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgradeWindows := getUpgradeWindows(tt.args.eventList)
			got := isInUpgradeWindow(upgradeWindows, tt.args.eventInterval)
			assert.Equal(t, tt.want, got)
		})
	}
}
