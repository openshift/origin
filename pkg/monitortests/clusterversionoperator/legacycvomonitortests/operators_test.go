package legacycvomonitortests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

// make_standard_upgrade_event_list creates a list of standard upgrade events using
// output from parsing the e2e-events.file
//
//	cat e2e-events_20240502-205107.json | jq '.items[] | \
//	   select(.source == "KubeEvent" and .locator.keys.clusterversion? == "cluster")| \
//	   "\(.from) \(.to) \(.message.reason)"'
func make_standard_upgrade_event_list(events []string) monitorapi.Intervals {
	var intervals monitorapi.Intervals
	for _, event := range events {
		parts := strings.Split(event, " ")
		if len(parts) < 3 {
			panic(fmt.Sprintf("Invalid event format: %s", event))
		}
		timeFrom, err := time.Parse(time.RFC3339, parts[0])
		if err != nil {
			panic(fmt.Sprintf("Error parsing From time: %v", err))
		}
		timeTo, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			panic(fmt.Sprintf("Error parsing To time: %v", err))
		}
		reasonString := parts[2]
		// If reason is not within the list of expected reasons, panic
		if reasonString != "UpgradeStarted" && reasonString != "UpgradeRollback" && reasonString != "UpgradeVersion" && reasonString != "UpgradeComplete" {
			panic(fmt.Sprintf("Unknown reason: %s\n", reasonString))
		}
		var reason monitorapi.IntervalReason
		switch reasonString {
		case "UpgradeStarted":
			reason = monitorapi.UpgradeStartedReason
		case "UpgradeRollback":
			reason = monitorapi.UpgradeRollbackReason
		case "UpgradeVersion":
			reason = monitorapi.UpgradeVersionReason
		case "UpgradeComplete":
			reason = monitorapi.UpgradeCompleteReason
		}
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
			From:   timeFrom,
			To:     timeTo,
		}
		intervals = append(intervals, interval)
	}
	return intervals
}

func Test_isInUpgradeWindow(t *testing.T) {
	type args struct {
		eventList     monitorapi.Intervals
		eventInterval monitorapi.Interval
	}

	test1_outside := monitorapi.Interval{
		From: time.Date(2024, 5, 1, 12, 49, 28, 0, time.UTC),
		To:   time.Date(2024, 5, 1, 12, 49, 28, 0, time.UTC),
	}
	test1_inside := monitorapi.Interval{
		From: time.Date(2024, 5, 1, 13, 44, 28, 0, time.UTC),
		To:   time.Date(2024, 5, 1, 13, 44, 28, 0, time.UTC),
	}

	test1_no_end := monitorapi.Interval{
		From: time.Date(2024, 5, 1, 14, 0, 0, 0, time.UTC),
		To:   time.Date(2024, 5, 1, 14, 0, 0, 0, time.UTC),
	}

	standardEventList := make_standard_upgrade_event_list([]string{
		"2024-05-01T12:51:09Z 2024-05-01T12:51:09Z UpgradeStarted",
		"2024-05-01T13:46:44Z 2024-05-01T13:46:44Z UpgradeVersion",
		"2024-05-01T13:46:44Z 2024-05-01T13:46:44Z UpgradeComplete",
	})

	eventListWithRollback := make_standard_upgrade_event_list([]string{
		"2024-05-01T22:21:42Z 2024-05-01T22:21:42Z UpgradeStarted",
		"2024-05-01T23:15:08Z 2024-05-01T23:15:08Z UpgradeRollback",
		"2024-05-02T00:11:18Z 2024-05-02T00:11:18Z UpgradeVersion",
		"2024-05-02T00:11:18Z 2024-05-02T00:11:18Z UpgradeComplete",
	})

	test2_outside_first := monitorapi.Interval{
		From: time.Date(2024, 5, 1, 22, 10, 0, 0, time.UTC),
		To:   time.Date(2024, 5, 1, 22, 10, 0, 0, time.UTC),
	}

	test2_inside_first := monitorapi.Interval{
		From: time.Date(2024, 5, 1, 22, 25, 0, 0, time.UTC),
		To:   time.Date(2024, 5, 1, 22, 25, 0, 0, time.UTC),
	}
	test2_inside_rollback := monitorapi.Interval{
		From: time.Date(2024, 5, 1, 23, 18, 0, 0, time.UTC),
		To:   time.Date(2024, 5, 1, 23, 18, 0, 0, time.UTC),
	}
	test2_past_end_of_rollback := monitorapi.Interval{
		From: time.Date(2024, 5, 2, 11, 20, 0, 0, time.UTC),
		To:   time.Date(2024, 5, 2, 11, 20, 0, 0, time.UTC),
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test 1a: single upgrade window, interval not within",
			args: args{
				eventList:     standardEventList,
				eventInterval: test1_outside,
			},
			want: false,
		},
		{
			name: "Test 1b: single upgrade window, interval within",
			args: args{
				eventList:     standardEventList,
				eventInterval: test1_inside,
			},
			want: true,
		},
		{
			name: "Test 1c: single upgrade window, with no end",
			args: args{
				eventList:     standardEventList[0:2],
				eventInterval: test1_no_end,
			},
			want: true,
		},
		{
			name: "Test 2a: upgrade with rollback, interval before first upgrade",
			args: args{
				eventList:     eventListWithRollback,
				eventInterval: test2_outside_first,
			},
			want: false,
		},
		{
			name: "Test 2b: upgrade with rollback, interval inside first upgrade",
			args: args{
				eventList:     eventListWithRollback,
				eventInterval: test2_inside_first,
			},
			want: true,
		},
		{
			name: "Test 2c: upgrade with rollback, interval inside rollback",
			args: args{
				eventList:     eventListWithRollback,
				eventInterval: test2_inside_rollback,
			},
			want: true,
		},
		{
			name: "Test 2d: upgrade with rollback, interval past end of rollback",
			args: args{
				eventList:     eventListWithRollback,
				eventInterval: test2_past_end_of_rollback,
			},
			want: false,
		},
		{
			name: "Test 2e: upgrade with rollback, interval past end of rollback with no end",
			args: args{
				eventList:     eventListWithRollback[0:3],
				eventInterval: test2_past_end_of_rollback,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInUpgradeWindow(tt.args.eventList, tt.args.eventInterval); got != tt.want {
				assert.Equal(t, tt.want, got)
				t.Errorf("isInUpgradeWindow() = %v, want %v", got, tt.want)
			}
		})
	}
}
