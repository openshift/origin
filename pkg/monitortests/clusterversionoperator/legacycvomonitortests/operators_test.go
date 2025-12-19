package legacycvomonitortests

import (
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/client-go/config/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
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

func Test_getOperatorsFromProgressingMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		expect  sets.Set[string]
	}{
		{
			name:    "unknown message	",
			message: "bar foo",
		},
		{
			name:    "single CO",
			message: "working towards ${VERSION}: 106 of 841 done (12% complete), waiting on single",
			expect:  sets.New[string]("single"),
		},
		{
			name:    "multiple COs",
			message: "working towards ${VERSION}: 106 of 841 done (12% complete), waiting on etcd, kube-apiserver",
			expect:  sets.New[string]("kube-apiserver", "etcd"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getOperatorsFromProgressingMessage(tt.message)
			assert.Equal(t, tt.expect, actual)
		})
	}
}

func Test_updateCOWaiting(t *testing.T) {
	now := time.Now()
	next := 3 * time.Hour
	interval := func(m string, start time.Time, d time.Duration) monitorapi.Interval {
		return monitorapi.NewInterval("foo", monitorapi.Warning).
			Locator(monitorapi.NewLocator().ClusterVersion(&configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "bar"}})).
			Message(monitorapi.NewMessage().Reason("reason").
				HumanMessage(m).
				WithAnnotation(monitorapi.AnnotationCondition, string(configv1.OperatorProgressing)).
				WithAnnotation(monitorapi.AnnotationStatus, string(configv1.ConditionTrue))).
			Build(start, start.Add(d))
	}

	tests := []struct {
		name    string
		message string
		d       time.Duration
		waiting map[string]monitorapi.Intervals
		expect  map[string]time.Duration
	}{
		{
			name:    "happy case",
			d:       time.Hour,
			message: "working towards ${VERSION}: 106 of 841 done (12% complete), waiting on etcd, kube-apiserver",
			waiting: map[string]monitorapi.Intervals{},
			expect:  map[string]time.Duration{"etcd": time.Hour, "kube-apiserver": time.Hour},
		},
		{
			name:    "incremental one",
			d:       2 * time.Minute,
			message: "working towards ${VERSION}: 106 of 841 done (12% complete), waiting on etcd, kube-apiserver",
			waiting: map[string]monitorapi.Intervals{"etcd": {interval("some", now, 3*time.Minute)}},
			expect:  map[string]time.Duration{"etcd": next + 2*time.Minute, "kube-apiserver": 2 * time.Minute},
		},
		{
			name:    "incremental all",
			d:       2 * time.Minute,
			message: "working towards ${VERSION}: 106 of 841 done (12% complete), waiting on etcd, kube-apiserver",
			waiting: map[string]monitorapi.Intervals{"etcd": {interval("some", now, 3*time.Minute)}, "kube-apiserver": {interval("some", now, 6*time.Minute)}},
			expect:  map[string]time.Duration{"etcd": next + 2*time.Minute, "kube-apiserver": next + 2*time.Minute},
		},
		{
			name:    "unknown message",
			message: "unknown message",
			waiting: map[string]monitorapi.Intervals{},
			expect:  map[string]time.Duration{},
		},
	}
	for _, tt := range tests {
		i := monitorapi.NewInterval(monitorapi.SourceVersionState, monitorapi.Warning).
			Locator(monitorapi.NewLocator().ClusterVersion(&configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"}})).
			Message(monitorapi.NewMessage().Reason("reason").
				HumanMessage(tt.message).
				WithAnnotation(monitorapi.AnnotationCondition, string(configv1.OperatorProgressing)).
				WithAnnotation(monitorapi.AnnotationStatus, string(configv1.ConditionTrue))).
			Build(now.Add(next), now.Add(next+tt.d))
		t.Run(tt.name, func(t *testing.T) {
			updateCOWaiting(i, tt.waiting)
			actual := map[string]time.Duration{}
			for co, intervals := range tt.waiting {
				from, to := fromAndTo(intervals)
				actual[co] = to.Sub(from)
			}
			assert.Equal(t, tt.expect, actual)
		})
	}
}

func Test_patchUpgradeWithConfigClient(t *testing.T) {
	tests := []struct {
		name         string
		cv           *configv1.ClusterVersion
		expect       bool
		expectErrMsg string
	}{
		{
			name:         "nil",
			cv:           &configv1.ClusterVersion{},
			expectErrMsg: "clusterversions.config.openshift.io \"version\" not found",
		},
		{
			name: "no history",
			cv: &configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
			},
			expectErrMsg: "not long enough (>1) history for versions in ClusterVersion/version for upgrade, found 0",
		},
		{
			name: "minor",
			cv: &configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status: configv1.ClusterVersionStatus{
					History: []configv1.UpdateHistory{
						{
							Version: "4.12.0",
						},
						{
							Version: "4.11.0",
						},
					},
				},
			},
		},
		{
			name: "minor",
			cv: &configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status: configv1.ClusterVersionStatus{
					History: []configv1.UpdateHistory{
						{
							Version: "4.11.1",
						},
						{
							Version: "4.11.0",
						},
					},
				},
			},
			expect: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, actualErr := patchUpgradeWithConfigClient(fake.NewClientset(tt.cv).ConfigV1())
			if tt.expectErrMsg != "" {
				assert.EqualError(t, actualErr, tt.expectErrMsg)
			} else {
				assert.Nil(t, actualErr)
			}
			assert.Equal(t, tt.expect, actual)
		})
	}
}
