package pathologicaleventlibrary

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

func Test_OverlapMatcherUsesFirstTimestamp(t *testing.T) {
	// Simulate the real-world scenario: an event fires 23 times over 90 minutes.
	// The interval's From is set to LastTimestamp (the final occurrence), but
	// firstTimestamp in annotations records when the event actually started.
	testStart := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	firstTimestamp := testStart.Add(15 * time.Minute) // 12:15
	lastTimestamp := testStart.Add(105 * time.Minute) // 13:45
	testEnd := testStart.Add(110 * time.Minute)       // 13:50

	// The "test interval" that the event should overlap with (12:00 - 13:50).
	testInterval := monitorapi.NewInterval(monitorapi.SourceE2ETest, monitorapi.Info).
		Locator(monitorapi.NewLocator().NodeFromName("test")).
		Message(monitorapi.NewMessage().HumanMessage("test interval")).
		Build(testStart, testEnd)

	// Build a pathological event interval the way watchevents/event.go does:
	// From = lastTimestamp, To = lastTimestamp + 1s, firstTimestamp in annotations.
	pathologicalInterval := monitorapi.NewInterval(monitorapi.SourceKubeEvent, monitorapi.Warning).
		Locator(monitorapi.NewLocator().PodFromNames("openshift-test", "test-pod", "")).
		Message(
			monitorapi.NewMessage().
				Reason("TestReason").
				HumanMessage("test pathological event").
				WithAnnotation(monitorapi.AnnotationCount, fmt.Sprintf("%d", 23)).
				WithAnnotation("firstTimestamp", firstTimestamp.Format(time.RFC3339)).
				WithAnnotation("lastTimestamp", lastTimestamp.Format(time.RFC3339))).
		Build(lastTimestamp, lastTimestamp.Add(1*time.Second))

	matcher := &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:               "TestMatcher",
			messageReasonRegex: regexp.MustCompile(`^TestReason$`),
		},
		allowIfWithinIntervals: monitorapi.Intervals{testInterval},
	}

	// Should be allowed because firstTimestamp (12:15) falls within the test interval (12:00-13:50).
	assert.True(t, matcher.Allows(pathologicalInterval, v1.HighlyAvailableTopologyMode),
		"event should be allowed when firstTimestamp falls within the overlap interval")

	// Now test with a test interval that ends before lastTimestamp but after firstTimestamp.
	// The event's firstTimestamp (12:15) is within [12:00, 13:00], and To (13:45:01) is NOT.
	shorterTestInterval := monitorapi.NewInterval(monitorapi.SourceE2ETest, monitorapi.Info).
		Locator(monitorapi.NewLocator().NodeFromName("test")).
		Message(monitorapi.NewMessage().HumanMessage("short test interval")).
		Build(testStart, testStart.Add(60*time.Minute)) // 12:00 - 13:00

	matcherShort := &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:               "TestMatcher",
			messageReasonRegex: regexp.MustCompile(`^TestReason$`),
		},
		allowIfWithinIntervals: monitorapi.Intervals{shorterTestInterval},
	}

	// Should NOT be allowed because the event's To (13:45:01) extends past the test interval's end (13:00).
	assert.False(t, matcherShort.Allows(pathologicalInterval, v1.HighlyAvailableTopologyMode),
		"event should not be allowed when its To extends beyond the overlap interval")

	// Test without firstTimestamp annotation — should fall back to From (lastTimestamp).
	noAnnotationInterval := monitorapi.NewInterval(monitorapi.SourceKubeEvent, monitorapi.Warning).
		Locator(monitorapi.NewLocator().PodFromNames("openshift-test", "test-pod", "")).
		Message(
			monitorapi.NewMessage().
				Reason("TestReason").
				HumanMessage("test pathological event").
				WithAnnotation(monitorapi.AnnotationCount, fmt.Sprintf("%d", 23))).
		Build(lastTimestamp, lastTimestamp.Add(1*time.Second))

	// Without firstTimestamp, From is lastTimestamp (13:45). The test interval [12:00, 13:50] contains it.
	assert.True(t, matcher.Allows(noAnnotationInterval, v1.HighlyAvailableTopologyMode),
		"event without firstTimestamp should fall back to From for overlap check")
}

func Test_singleEventThresholdCheck_getNamespacedFailuresAndFlakes(t *testing.T) {
	namespace := "openshift-etcd-operator"
	samplePod := "etcd-operator-6f9b4d9d4f-4q9q8"

	testName := "[sig-cluster-lifecycle] pathological event should not see excessive Back-off restarting failed containers"
	backoffMatcher := NewSingleEventThresholdCheck(testName, AllowBackOffRestartingFailedContainer,
		DuplicateEventThreshold, BackoffRestartingFlakeThreshold)
	type fields struct {
		testName       string
		matcher        *SimplePathologicalEventMatcher
		failThreshold  int
		flakeThreshold int
	}
	type args struct {
		events monitorapi.Intervals
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		expectedKeyCount int
	}{
		{
			name: "Successful test yields no keys",
			fields: fields{
				testName:       testName,
				matcher:        backoffMatcher.matcher,
				failThreshold:  DuplicateEventThreshold,
				flakeThreshold: BackoffRestartingFlakeThreshold,
			},
			args: args{
				events: monitorapi.Intervals{
					BuildTestDupeKubeEvent(namespace, samplePod,
						"BackOff",
						"Back-off restarting failed container",
						5),
				},
			},
			expectedKeyCount: 0,
		},
		{
			name: "Failing test yields one key",
			fields: fields{
				testName:       testName,
				matcher:        backoffMatcher.matcher,
				failThreshold:  DuplicateEventThreshold,
				flakeThreshold: BackoffRestartingFlakeThreshold,
			},
			args: args{
				events: monitorapi.Intervals{
					BuildTestDupeKubeEvent(namespace, samplePod,
						"BackOff",
						"Back-off restarting failed container",
						21),
				},
			},
			expectedKeyCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &singleEventThresholdCheck{
				testName:       tt.fields.testName,
				matcher:        tt.fields.matcher,
				failThreshold:  tt.fields.failThreshold,
				flakeThreshold: tt.fields.flakeThreshold,
			}
			got := s.getNamespacedFailuresAndFlakes(tt.args.events)
			assert.Equal(t, tt.expectedKeyCount, len(got))
		})
	}
}
