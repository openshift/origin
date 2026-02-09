package etcdloganalyzer

import (
	"strconv"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/podaccess"
	"k8s.io/apimachinery/pkg/runtime"
)

// mockRecorder captures intervals added to it for testing
type mockRecorder struct {
	intervals monitorapi.Intervals
}

func (m *mockRecorder) AddIntervals(eventIntervals ...monitorapi.Interval) {
	m.intervals = append(m.intervals, eventIntervals...)
}

func (m *mockRecorder) StartInterval(interval monitorapi.Interval) int { return 0 }
func (m *mockRecorder) EndInterval(startedInterval int, t time.Time) *monitorapi.Interval {
	return nil
}
func (m *mockRecorder) Record(conditions ...monitorapi.Condition)                {}
func (m *mockRecorder) RecordAt(t time.Time, conditions ...monitorapi.Condition) {}
func (m *mockRecorder) RecordResource(resourceType string, obj runtime.Object)   {}

func TestEtcdRecorderBatching(t *testing.T) {
	mock := &mockRecorder{}
	recorder := newEtcdRecorder(mock)

	baseTime := time.Date(2026, 2, 3, 10, 51, 0, 0, time.UTC)
	locator := monitorapi.Locator{
		Type: monitorapi.LocatorTypeContainer,
		Keys: map[monitorapi.LocatorKey]string{
			"node":      "node-1",
			"pod":       "etcd-node-1",
			"container": "etcd",
			"namespace": "openshift-etcd",
		},
	}

	// Simulate 5 "apply request took too long" messages in the same minute
	for i := 0; i < 5; i++ {
		ts := baseTime.Add(time.Duration(i*10) * time.Second).Format(time.RFC3339)
		logLine := podaccess.LogLineContent{
			Locator: locator,
			Instant: baseTime.Add(time.Duration(i*10) * time.Second),
			Line:    `{"level":"warn","ts":"` + ts + `","msg":"apply request took too long"}`,
		}
		recorder.HandleLogLine(logLine)
	}

	// Simulate 3 "slow fdatasync" messages in the same minute
	for i := 0; i < 3; i++ {
		ts := baseTime.Add(time.Duration(i*15) * time.Second).Format(time.RFC3339)
		logLine := podaccess.LogLineContent{
			Locator: locator,
			Instant: baseTime.Add(time.Duration(i*15) * time.Second),
			Line:    `{"level":"warn","ts":"` + ts + `","msg":"slow fdatasync"}`,
		}
		recorder.HandleLogLine(logLine)
	}

	// Simulate 2 "apply request took too long" messages in a different minute
	for i := 0; i < 2; i++ {
		ts := baseTime.Add(time.Minute).Add(time.Duration(i*10) * time.Second).Format(time.RFC3339)
		logLine := podaccess.LogLineContent{
			Locator: locator,
			Instant: baseTime.Add(time.Minute).Add(time.Duration(i*10) * time.Second),
			Line:    `{"level":"warn","ts":"` + ts + `","msg":"apply request took too long"}`,
		}
		recorder.HandleLogLine(logLine)
	}

	// Before flush, no intervals should be recorded
	if len(mock.intervals) != 0 {
		t.Errorf("Expected 0 intervals before Flush, got %d", len(mock.intervals))
	}

	// Flush the batches
	recorder.Flush()

	// After flush, we should have 3 batched intervals:
	// - "apply request took too long" at 10:51 with count=5
	// - "slow fdatasync" at 10:51 with count=3
	// - "apply request took too long" at 10:52 with count=2
	if len(mock.intervals) != 3 {
		t.Errorf("Expected 3 intervals after Flush, got %d", len(mock.intervals))
	}

	// Verify the total count across all batches equals the original count
	totalCount := 0
	for _, interval := range mock.intervals {
		countStr := interval.Message.Annotations[monitorapi.AnnotationCount]
		count, err := strconv.Atoi(countStr)
		if err != nil {
			t.Errorf("Invalid count annotation %q: %v", countStr, err)
			continue
		}
		totalCount += count

		// Verify Display is true
		if !interval.Display {
			t.Errorf("Expected Display=true for batched interval")
		}

		// Verify source is EtcdLog
		if interval.Source != monitorapi.SourceEtcdLog {
			t.Errorf("Expected source EtcdLog, got %s", interval.Source)
		}

		// Verify time is minute-aligned
		if interval.From.Second() != 0 || interval.From.Nanosecond() != 0 {
			t.Errorf("Expected minute-aligned From time, got %v", interval.From)
		}
	}

	expectedTotalCount := 5 + 3 + 2
	if totalCount != expectedTotalCount {
		t.Errorf("Expected total count %d, got %d", expectedTotalCount, totalCount)
	}

	// Verify the locator includes the etcd-event key for chart separation
	for _, interval := range mock.intervals {
		eventKey, ok := interval.Locator.Keys["etcd-event"]
		if !ok {
			t.Errorf("Expected locator to have etcd-event key, got keys: %v", interval.Locator.Keys)
			continue
		}
		// Verify the event key is one of the expected values
		validKeys := map[string]bool{
			"apply-slow":    true,
			"slow-fdatasync": true,
		}
		if !validKeys[eventKey] {
			t.Errorf("Unexpected etcd-event key: %s", eventKey)
		}
	}
}

func TestEtcdRecorderLeadershipMessagesNotBatched(t *testing.T) {
	mock := &mockRecorder{}
	recorder := newEtcdRecorder(mock)

	baseTime := time.Date(2026, 2, 3, 10, 51, 0, 0, time.UTC)
	locator := monitorapi.Locator{
		Type: monitorapi.LocatorTypeContainer,
		Keys: map[monitorapi.LocatorKey]string{
			"node":      "node-1",
			"pod":       "etcd-node-1",
			"container": "etcd",
			"namespace": "openshift-etcd",
		},
	}

	// Leadership messages should be recorded immediately, not batched
	logLine := podaccess.LogLineContent{
		Locator: locator,
		Instant: baseTime,
		Line:    `{"level":"info","ts":"2026-02-03T10:51:00Z","msg":"raft.node: abc123 elected leader def456 at term 5"}`,
	}
	recorder.HandleLogLine(logLine)

	// Leadership message should be recorded immediately (not in batch)
	if len(mock.intervals) != 1 {
		t.Errorf("Expected 1 interval for leadership message, got %d", len(mock.intervals))
	}

	// Verify it's a leadership event, not a batched etcd log
	if len(mock.intervals) > 0 {
		if mock.intervals[0].Source != monitorapi.SourceEtcdLeadership {
			t.Errorf("Expected source EtcdLeadership, got %s", mock.intervals[0].Source)
		}
		if mock.intervals[0].Message.Reason != "LeaderFound" {
			t.Errorf("Expected reason LeaderFound, got %s", mock.intervals[0].Message.Reason)
		}
	}
}

func TestEtcdRecorderFlushClearsBatches(t *testing.T) {
	mock := &mockRecorder{}
	recorder := newEtcdRecorder(mock)

	baseTime := time.Date(2026, 2, 3, 10, 51, 0, 0, time.UTC)
	locator := monitorapi.Locator{
		Type: monitorapi.LocatorTypeContainer,
		Keys: map[monitorapi.LocatorKey]string{
			"node": "node-1",
		},
	}

	// Add some log lines
	ts := baseTime.Format(time.RFC3339)
	logLine := podaccess.LogLineContent{
		Locator: locator,
		Instant: baseTime,
		Line:    `{"level":"warn","ts":"` + ts + `","msg":"apply request took too long"}`,
	}
	recorder.HandleLogLine(logLine)

	// First flush
	recorder.Flush()
	if len(mock.intervals) != 1 {
		t.Errorf("Expected 1 interval after first Flush, got %d", len(mock.intervals))
	}

	// Second flush should not add any more intervals (batches were cleared)
	recorder.Flush()
	if len(mock.intervals) != 1 {
		t.Errorf("Expected still 1 interval after second Flush, got %d", len(mock.intervals))
	}
}
