package etcdloganalyzer

import (
	_ "embed"
	"strconv"
	"testing"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
)

//go:embed testdata/etcdlog_test_intervals.json
var testIntervalsJSON []byte

func TestBatchEtcdLogIntervals(t *testing.T) {
	// Parse the embedded test data
	allIntervals, err := monitorserialization.IntervalsFromJSON(testIntervalsJSON)
	if err != nil {
		t.Fatalf("Failed to parse embedded test intervals: %v", err)
	}

	// Filter to only batchable EtcdLog intervals and count them
	batchableIntervals := allIntervals.Filter(func(i monitorapi.Interval) bool {
		return i.Source == monitorapi.SourceEtcdLog &&
			i.Message.Annotations[monitorapi.AnnotationBatchable] == "true"
	})
	originalCount := len(batchableIntervals)
	t.Logf("Original batchable EtcdLog interval count: %d", originalCount)

	// Count non-batchable EtcdLog intervals (like leadership events)
	nonBatchableEtcdIntervals := allIntervals.Filter(func(i monitorapi.Interval) bool {
		return i.Source == monitorapi.SourceEtcdLog &&
			i.Message.Annotations[monitorapi.AnnotationBatchable] != "true"
	})
	t.Logf("Non-batchable EtcdLog intervals (should be unchanged): %d", len(nonBatchableEtcdIntervals))

	if originalCount == 0 {
		t.Fatal("No batchable EtcdLog intervals found in test data")
	}

	// Apply the batching algorithm
	batchedIntervals := BatchEtcdLogIntervals(allIntervals)
	batchedCount := len(batchedIntervals)
	t.Logf("Batched interval count: %d", batchedCount)

	// Tally up the counts from the batched intervals
	var talliedCount int
	for _, interval := range batchedIntervals {
		countStr, ok := interval.Message.Annotations[monitorapi.AnnotationCount]
		if !ok {
			t.Errorf("Batched interval missing count annotation: %v", interval)
			talliedCount++
			continue
		}
		count, err := strconv.Atoi(countStr)
		if err != nil {
			t.Errorf("Invalid count annotation %q: %v", countStr, err)
			talliedCount++
			continue
		}
		talliedCount += count
	}

	t.Logf("Tallied count from batched intervals: %d", talliedCount)

	// Verify the tallied count matches the original batchable count
	if talliedCount != originalCount {
		t.Errorf("Count mismatch: original batchable=%d, tallied=%d", originalCount, talliedCount)
	}

	// Verify all batched intervals are EtcdLog source
	for _, interval := range batchedIntervals {
		if interval.Source != monitorapi.SourceEtcdLog {
			t.Errorf("Batched interval has wrong source: got %s, want EtcdLog", interval.Source)
		}
	}

	// Verify the batchable annotation is removed from all batched intervals
	for _, interval := range batchedIntervals {
		if _, hasBatchable := interval.Message.Annotations[monitorapi.AnnotationBatchable]; hasBatchable {
			t.Errorf("Batched interval still has batchable annotation: %v", interval)
		}
	}

	// Verify batched intervals have proper 1-minute aligned times
	for _, interval := range batchedIntervals {
		if interval.From.Second() != 0 || interval.From.Nanosecond() != 0 {
			t.Errorf("Batched interval From time not aligned to minute boundary: %v", interval.From)
		}
	}

	// Verify compression happened (we should have fewer batched intervals than original)
	if batchedCount >= originalCount {
		t.Errorf("Expected compression: batchedCount=%d should be less than originalCount=%d", batchedCount, originalCount)
	}

	// Test the expected batching based on our test data (only batchable intervals):
	// - node-1, "apply request took too long", minute 10:51 -> 3 intervals batched
	// - node-1, "apply request took too long", minute 10:52 -> 2 intervals batched
	// - node-1, "slow fdatasync", minute 10:51 -> 2 intervals batched
	// - node-2, "apply request took too long", minute 10:51 -> 2 intervals batched
	// - node-2, "waiting for ReadIndex...", minute 10:51 -> 1 interval
	// - node-3, "apply request took too long", minute 10:51 -> 1 interval
	// Note: "restarting local member" is NOT batchable (no batchable annotation)
	// Total: 6 batched intervals from 11 original batchable EtcdLog intervals

	expectedBatchedCount := 6
	if batchedCount != expectedBatchedCount {
		t.Errorf("Expected %d batched intervals, got %d", expectedBatchedCount, batchedCount)
	}

	// Verify specific batch counts by building a map
	type batchKey struct {
		node         string
		humanMessage string
		minute       int
	}
	expectedCounts := map[batchKey]int{
		{"node-1", "apply request took too long", 51}:                            3,
		{"node-1", "apply request took too long", 52}:                            2,
		{"node-1", "slow fdatasync", 51}:                                         2,
		{"node-2", "apply request took too long", 51}:                            2,
		{"node-2", "waiting for ReadIndex response took too long, retrying", 51}: 1,
		{"node-3", "apply request took too long", 51}:                            1,
	}

	for _, interval := range batchedIntervals {
		node := interval.Locator.Keys[monitorapi.LocatorNodeKey]
		minute := interval.From.Minute()
		key := batchKey{node, interval.Message.HumanMessage, minute}

		expectedCount, ok := expectedCounts[key]
		if !ok {
			t.Errorf("Unexpected batch: node=%s, message=%s, minute=%d", node, interval.Message.HumanMessage, minute)
			continue
		}

		actualCount, _ := strconv.Atoi(interval.Message.Annotations[monitorapi.AnnotationCount])
		if actualCount != expectedCount {
			t.Errorf("Wrong count for batch (node=%s, message=%s, minute=%d): got %d, want %d",
				node, interval.Message.HumanMessage, minute, actualCount, expectedCount)
		}
	}
}

func TestBatchEtcdLogIntervals_EmptyInput(t *testing.T) {
	result := BatchEtcdLogIntervals(monitorapi.Intervals{})
	if len(result) != 0 {
		t.Errorf("Expected empty result for empty input, got %d intervals", len(result))
	}
}

func TestBatchEtcdLogIntervals_NoEtcdLogIntervals(t *testing.T) {
	// Create intervals with a different source
	intervals := monitorapi.Intervals{
		{
			Condition: monitorapi.Condition{
				Level: monitorapi.Info,
			},
			Source: monitorapi.SourcePodMonitor,
		},
	}

	result := BatchEtcdLogIntervals(intervals)
	if len(result) != 0 {
		t.Errorf("Expected empty result for non-EtcdLog intervals, got %d intervals", len(result))
	}
}

func TestBatchEtcdLogIntervals_NonBatchableIntervalsIgnored(t *testing.T) {
	// Create EtcdLog intervals without the batchable annotation (like leadership events)
	intervals := monitorapi.Intervals{
		{
			Condition: monitorapi.Condition{
				Level: monitorapi.Warning,
				Message: monitorapi.Message{
					Reason:       "LeaderElected",
					HumanMessage: "became leader at term 5",
					Annotations:  map[monitorapi.AnnotationKey]string{},
				},
			},
			Source: monitorapi.SourceEtcdLog,
		},
	}

	result := BatchEtcdLogIntervals(intervals)
	if len(result) != 0 {
		t.Errorf("Expected empty result for non-batchable EtcdLog intervals, got %d intervals", len(result))
	}
}
