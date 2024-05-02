package legacycvomonitortests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

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

	standardEventList := monitorapi.Intervals{
		monitorapi.Interval{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorClusterVersionKey: "cluster",
					},
				},
				Message: monitorapi.Message{
					Reason: "UpgradeStarted",
				},
			},
			Source: monitorapi.SourceKubeEvent,
			From:   time.Date(2024, 5, 1, 12, 51, 9, 0, time.UTC),
			To:     time.Date(2024, 5, 1, 12, 51, 9, 0, time.UTC),
		},
		monitorapi.Interval{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorClusterVersionKey: "cluster",
					},
				},
				Message: monitorapi.Message{
					Reason: "UpgradeVersion",
				},
			},
			Source: monitorapi.SourceKubeEvent,
			From:   time.Date(2024, 5, 1, 13, 46, 44, 0, time.UTC),
			To:     time.Date(2024, 5, 1, 13, 46, 44, 0, time.UTC),
		},
		monitorapi.Interval{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorClusterVersionKey: "cluster",
					},
				},
				Message: monitorapi.Message{
					Reason: "UpgradeComplete",
				},
			},
			Source: monitorapi.SourceKubeEvent,
			From:   time.Date(2024, 5, 1, 13, 46, 44, 0, time.UTC),
			To:     time.Date(2024, 5, 1, 13, 46, 44, 0, time.UTC),
		},
	}

	eventListWithRollback := monitorapi.Intervals{
		monitorapi.Interval{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorClusterVersionKey: "cluster",
					},
				},
				Message: monitorapi.Message{
					Reason: "UpgradeStarted",
				},
			},
			Source: monitorapi.SourceKubeEvent,
			From:   time.Date(2024, 5, 1, 22, 21, 42, 0, time.UTC),
			To:     time.Date(2024, 5, 1, 22, 21, 42, 0, time.UTC),
		},
		monitorapi.Interval{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorClusterVersionKey: "cluster",
					},
				},
				Message: monitorapi.Message{
					Reason: "UpgradeRollback",
					Annotations: map[monitorapi.AnnotationKey]string{
						"reason": "UpgradeRollback",
					},
				},
			},
			Source: monitorapi.SourceKubeEvent,
			From:   time.Date(2024, 5, 1, 23, 15, 8, 0, time.UTC),
			To:     time.Date(2024, 5, 1, 23, 15, 8, 0, time.UTC),
		},
		monitorapi.Interval{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorClusterVersionKey: "cluster",
					},
				},
				Message: monitorapi.Message{
					Reason: "UpgradeVersion",
					Annotations: map[monitorapi.AnnotationKey]string{
						"reason": "UpgradeVersion",
					},
				},
			},
			Source: monitorapi.SourceKubeEvent,
			From:   time.Date(2024, 5, 2, 0, 11, 18, 0, time.UTC),
			To:     time.Date(2024, 5, 2, 0, 11, 18, 0, time.UTC),
		},
		monitorapi.Interval{
			Condition: monitorapi.Condition{
				Locator: monitorapi.Locator{
					Keys: map[monitorapi.LocatorKey]string{
						monitorapi.LocatorClusterVersionKey: "cluster",
					},
				},
				Message: monitorapi.Message{
					Reason: "UpgradeComplete",
				},
			},
			Source:  monitorapi.SourceKubeEvent,
			Display: true,
			From:    time.Date(2024, 5, 2, 0, 11, 18, 0, time.UTC),
			To:      time.Date(2024, 5, 2, 0, 11, 18, 0, time.UTC),
		},
	}
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
