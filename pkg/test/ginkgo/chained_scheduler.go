package ginkgo

import (
	"context"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

// TestBucket represents a group of tests with a specific max parallelism level
type TestBucket struct {
	Name           string
	Tests          []*testCase
	MaxParallelism int
}

// chainedBucketScheduler manages multiple test buckets with progressive chaining.
// It activates buckets progressively: when bucket A's active count drops below
// bucket B's max parallelism, bucket B starts feeding tests.
type chainedBucketScheduler struct {
	mu                   sync.Mutex
	cond                 *sync.Cond
	buckets              []TestBucket
	testQueues           [][]*testCase        // one queue per bucket
	testToBucket         map[*testCase]int    // maps test to bucket index
	activeTestCounts     map[int]int          // bucket index -> active test count
	runningConflicts     map[string]sets.Set[string] // tracks conflicts per group
	activeTaints         map[string]int       // tracks active taints
	currentBucketIdx     int                  // primary active bucket
	monitorEventRecorder monitorapi.Recorder
	intervalIDs          map[int]int       // bucket index -> interval ID
	startTimes           map[int]time.Time // bucket index -> start time
}

// newChainedBucketScheduler creates a scheduler that chains multiple test buckets
func newChainedBucketScheduler(buckets []TestBucket, monitorEventRecorder monitorapi.Recorder) *chainedBucketScheduler {
	testQueues := make([][]*testCase, len(buckets))
	testToBucket := make(map[*testCase]int)

	for i, bucket := range buckets {
		testQueues[i] = make([]*testCase, len(bucket.Tests))
		copy(testQueues[i], bucket.Tests)
		for _, test := range bucket.Tests {
			testToBucket[test] = i
		}
	}

	cbs := &chainedBucketScheduler{
		buckets:              buckets,
		testQueues:           testQueues,
		testToBucket:         testToBucket,
		currentBucketIdx:     0,
		activeTestCounts:     make(map[int]int),
		runningConflicts:     make(map[string]sets.Set[string]),
		activeTaints:         make(map[string]int),
		monitorEventRecorder: monitorEventRecorder,
		intervalIDs:          make(map[int]int),
		startTimes:           make(map[int]time.Time),
	}
	cbs.cond = sync.NewCond(&cbs.mu)

	// Start the first bucket
	if len(buckets) > 0 {
		cbs.startBucket(0)
	}

	return cbs
}

// startBucket activates a bucket by starting its monitoring interval
func (cbs *chainedBucketScheduler) startBucket(idx int) {
	if idx >= len(cbs.buckets) {
		return
	}

	bucket := cbs.buckets[idx]
	intervalID, startTime := recordTestBucketInterval(cbs.monitorEventRecorder, bucket.Name)
	cbs.intervalIDs[idx] = intervalID
	cbs.startTimes[idx] = startTime
	logrus.Infof("Started bucket %s (index %d) with max parallelism %d", bucket.Name, idx, bucket.MaxParallelism)
}

// endBucket finishes a bucket's monitoring interval
func (cbs *chainedBucketScheduler) endBucket(idx int) {
	if intervalID, ok := cbs.intervalIDs[idx]; ok {
		cbs.monitorEventRecorder.EndInterval(intervalID, time.Now())
		if startTime, ok := cbs.startTimes[idx]; ok {
			logrus.Infof("Completed %s bucket in %v", cbs.buckets[idx].Name, time.Since(startTime))
		}
		delete(cbs.intervalIDs, idx)
		delete(cbs.startTimes, idx)
	}
}

// shouldActivateNextBucket determines if we should start pulling from the next bucket
func (cbs *chainedBucketScheduler) shouldActivateNextBucket() bool {
	if cbs.currentBucketIdx >= len(cbs.buckets)-1 {
		return false
	}

	currentActive := cbs.activeTestCounts[cbs.currentBucketIdx]
	nextBucket := cbs.buckets[cbs.currentBucketIdx+1]

	return currentActive < nextBucket.MaxParallelism
}

// canTolerateTaints checks if a test can tolerate all currently active taints
func (cbs *chainedBucketScheduler) canTolerateTaints(test *testCase) bool {
	if test.spec == nil {
		return len(cbs.activeTaints) == 0
	}

	for taint, count := range cbs.activeTaints {
		if count <= 0 {
			continue
		}

		tolerated := false
		for _, toleration := range test.spec.Resources.Isolation.Toleration {
			if toleration == taint {
				tolerated = true
				break
			}
		}
		if !tolerated {
			return false
		}
	}
	return true
}

// findRunnableTest scans a bucket's queue for a runnable test
func (cbs *chainedBucketScheduler) findRunnableTest(bucketIdx int) *testCase {
	queue := cbs.testQueues[bucketIdx]

	for i, test := range queue {
		conflictGroup := getTestConflictGroup(test)

		if cbs.runningConflicts[conflictGroup] == nil {
			cbs.runningConflicts[conflictGroup] = sets.New[string]()
		}

		// Check conflicts
		hasConflict := false
		if test.spec != nil {
			for _, conflict := range test.spec.Resources.Isolation.Conflict {
				if cbs.runningConflicts[conflictGroup].Has(conflict) {
					hasConflict = true
					break
				}
			}
		}

		// Check taints
		canTolerate := cbs.canTolerateTaints(test)

		if !hasConflict && canTolerate {
			// Found a runnable test - mark it as running
			if test.spec != nil {
				for _, conflict := range test.spec.Resources.Isolation.Conflict {
					cbs.runningConflicts[conflictGroup].Insert(conflict)
				}
				for _, taint := range test.spec.Resources.Isolation.Taint {
					cbs.activeTaints[taint]++
				}
			}

			// Remove from queue
			cbs.testQueues[bucketIdx] = append(queue[:i], queue[i+1:]...)
			return test
		}
	}

	return nil
}

// GetNextTestToRun implements TestScheduler interface
func (cbs *chainedBucketScheduler) GetNextTestToRun(ctx context.Context) *testCase {
	cbs.mu.Lock()
	defer cbs.mu.Unlock()

	for {
		if ctx.Err() != nil {
			return nil
		}

		// Check if we've gone past all buckets
		if cbs.currentBucketIdx >= len(cbs.buckets) {
			// Check if there are still active tests from earlier buckets
			hasActiveTests := false
			for _, count := range cbs.activeTestCounts {
				if count > 0 {
					hasActiveTests = true
					break
				}
			}
			if !hasActiveTests {
				return nil
			}
			// Wait for active tests to complete
			cbs.cond.Wait()
			continue
		}

		// Calculate global max parallelism - it's the minimum of all active buckets' max parallelism
		globalMaxParallelism := cbs.buckets[cbs.currentBucketIdx].MaxParallelism
		for i := cbs.currentBucketIdx + 1; i < len(cbs.buckets); i++ {
			if _, started := cbs.startTimes[i]; started {
				// This bucket is active, so global limit is constrained by it
				if cbs.buckets[i].MaxParallelism < globalMaxParallelism {
					globalMaxParallelism = cbs.buckets[i].MaxParallelism
				}
			}
		}

		// Calculate total active tests across all buckets
		totalActive := 0
		for _, count := range cbs.activeTestCounts {
			totalActive += count
		}

		// If we've hit the global parallelism cap, wait for tests to complete
		if totalActive >= globalMaxParallelism {
			// Check if all buckets are done first
			allDone := cbs.currentBucketIdx >= len(cbs.buckets)
			if !allDone {
				hasActiveTests := false
				hasRemainingTests := false
				for i := cbs.currentBucketIdx; i < len(cbs.buckets); i++ {
					if cbs.activeTestCounts[i] > 0 {
						hasActiveTests = true
					}
					if len(cbs.testQueues[i]) > 0 {
						hasRemainingTests = true
					}
				}
				if !hasActiveTests && !hasRemainingTests {
					allDone = true
				}
			}

			if allDone {
				return nil
			}

			// Wait for a test to complete to free up a slot
			cbs.cond.Wait()
			continue
		}

		// Try to get a test from any active bucket
		for bucketIdx := cbs.currentBucketIdx; bucketIdx < len(cbs.buckets); bucketIdx++ {
			// Only consider buckets that:
			// 1. Have tests remaining
			// 2. Haven't exceeded their individual max parallelism
			if len(cbs.testQueues[bucketIdx]) == 0 {
				continue
			}

			bucket := cbs.buckets[bucketIdx]
			if cbs.activeTestCounts[bucketIdx] >= bucket.MaxParallelism {
				continue
			}

			// Try to find a runnable test in this bucket
			test := cbs.findRunnableTest(bucketIdx)
			if test != nil {
				cbs.activeTestCounts[bucketIdx]++

				// Check if we should activate the next bucket
				if bucketIdx == cbs.currentBucketIdx && cbs.shouldActivateNextBucket() {
					nextIdx := cbs.currentBucketIdx + 1
					if _, started := cbs.startTimes[nextIdx]; !started {
						cbs.startBucket(nextIdx)
					}
				}

				return test
			}
		}

		// Check if current bucket is exhausted and has no active tests
		if cbs.currentBucketIdx < len(cbs.buckets) {
			if len(cbs.testQueues[cbs.currentBucketIdx]) == 0 && cbs.activeTestCounts[cbs.currentBucketIdx] == 0 {
				cbs.endBucket(cbs.currentBucketIdx)
				delete(cbs.activeTestCounts, cbs.currentBucketIdx)
				cbs.currentBucketIdx++

				// Start the new current bucket
				if cbs.currentBucketIdx < len(cbs.buckets) {
					cbs.startBucket(cbs.currentBucketIdx)
					continue // Try again with new bucket
				}
			}
		}

		// Check if all buckets are done
		allDone := cbs.currentBucketIdx >= len(cbs.buckets)
		if !allDone {
			// Check if we have any active tests or remaining tests
			hasActiveTests := false
			hasRemainingTests := false
			for i := cbs.currentBucketIdx; i < len(cbs.buckets); i++ {
				if cbs.activeTestCounts[i] > 0 {
					hasActiveTests = true
				}
				if len(cbs.testQueues[i]) > 0 {
					hasRemainingTests = true
				}
			}
			if !hasActiveTests && !hasRemainingTests {
				allDone = true
			}
		}

		if allDone {
			return nil
		}

		// Wait for a test to complete
		cbs.cond.Wait()
	}
}

// MarkTestComplete implements TestScheduler interface
func (cbs *chainedBucketScheduler) MarkTestComplete(test *testCase) {
	cbs.mu.Lock()
	defer cbs.mu.Unlock()

	// Find which bucket this test belongs to
	bucketIdx, ok := cbs.testToBucket[test]
	if !ok {
		cbs.cond.Broadcast()
		return
	}

	// Clean up conflicts and taints
	if test.spec != nil {
		conflictGroup := getTestConflictGroup(test)
		if groupConflicts, exists := cbs.runningConflicts[conflictGroup]; exists {
			for _, conflict := range test.spec.Resources.Isolation.Conflict {
				groupConflicts.Delete(conflict)
			}
		}

		for _, taint := range test.spec.Resources.Isolation.Taint {
			cbs.activeTaints[taint]--
			if cbs.activeTaints[taint] <= 0 {
				delete(cbs.activeTaints, taint)
			}
		}
	}

	// Decrement active count
	if cbs.activeTestCounts[bucketIdx] > 0 {
		cbs.activeTestCounts[bucketIdx]--

		// Check if we should activate the next bucket
		if bucketIdx == cbs.currentBucketIdx && cbs.shouldActivateNextBucket() {
			nextIdx := cbs.currentBucketIdx + 1
			if nextIdx < len(cbs.buckets) {
				if _, started := cbs.startTimes[nextIdx]; !started {
					cbs.startBucket(nextIdx)
				}
			}
		}
	}

	// Signal waiting workers
	cbs.cond.Broadcast()
}
