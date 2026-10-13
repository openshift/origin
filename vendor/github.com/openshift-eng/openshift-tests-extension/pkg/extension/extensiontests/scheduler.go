package extensiontests

import (
	"context"
	"sync"

	"github.com/openshift-eng/openshift-tests-extension/pkg/util/sets"
)

const defaultConflictGroup = "default"

// Scheduler defines the interface for test scheduling.
// It manages scheduling based on isolation requirements (conflicts, taints, tolerations).
//
// Callers must follow a get-once, complete-once protocol: every non-nil spec returned by
// GetNextTestToRun must eventually be passed to MarkTestComplete exactly once, including
// when test execution panics.
type Scheduler interface {
	// GetNextTestToRun blocks until a test is available, then returns it.
	// Returns nil when all tests have been distributed (queue is empty) or context is cancelled.
	// When a test is returned, it is atomically removed from queue and marked as running.
	// This method can be safely called from multiple goroutines concurrently.
	GetNextTestToRun(ctx context.Context) *ExtensionTestSpec

	// MarkTestComplete marks a test as complete, cleaning up its conflicts and taints.
	// This may unblock other tests that were waiting.
	// This method can be safely called from multiple goroutines concurrently.
	MarkTestComplete(spec *ExtensionTestSpec)
}

// testScheduler manages test scheduling based on conflicts, taints, and tolerations.
// It maintains an ordered queue of tests and provides thread-safe scheduling operations.
type testScheduler struct {
	mu               sync.Mutex
	cond             *sync.Cond // condition variable to signal when tests complete
	tests            []*ExtensionTestSpec
	runningConflicts map[string]sets.Set[string] // tracks which conflicts are running per group: group -> set of conflicts
	activeTaints     map[string]int              // tracks how many tests are currently applying each taint
}

// NewScheduler creates a test scheduler. It accepts tests in any order and schedules
// them based on isolation requirements (conflicts, taints, tolerations).
func NewScheduler(tests []*ExtensionTestSpec) Scheduler {
	ts := &testScheduler{
		tests:            append([]*ExtensionTestSpec(nil), tests...),
		runningConflicts: make(map[string]sets.Set[string]),
		activeTaints:     make(map[string]int),
	}
	ts.cond = sync.NewCond(&ts.mu)
	return ts
}

// GetNextTestToRun blocks until a test is available to run, or returns nil
// if all tests have been distributed or the context is cancelled.
// It continuously scans the queue and waits for state changes when no tests are runnable.
// When a test is returned, it is atomically removed from queue and marked as running.
func (ts *testScheduler) GetNextTestToRun(ctx context.Context) *ExtensionTestSpec {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Set up context cancellation to wake up any waiting goroutine
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			ts.mu.Lock()
			ts.cond.Broadcast()
			ts.mu.Unlock()
		case <-done:
			// Normal exit, nothing to do
		}
	}()

	for {
		// Check if context is cancelled
		if ctx.Err() != nil {
			return nil
		}

		// Check if all tests have been distributed
		if len(ts.tests) == 0 {
			return nil
		}

		// Scan from beginning to find first runnable test
		for i, spec := range ts.tests {
			conflictGroup := getConflictGroup(spec)

			// Ensure the conflict group set exists
			if ts.runningConflicts[conflictGroup] == nil {
				ts.runningConflicts[conflictGroup] = sets.New[string]()
			}

			// Check if any of the test's conflicts are currently running within its group
			hasConflict := ts.hasActiveConflict(spec, conflictGroup)

			// Check if test can tolerate all currently active taints
			canTolerate := ts.canTolerateTaints(spec)

			if !hasConflict && canTolerate {
				isolation := &spec.Resources.Isolation

				// Found a runnable test - ATOMICALLY:
				// 1. Mark conflicts as running
				for _, conflict := range isolation.Conflict {
					ts.runningConflicts[conflictGroup].Insert(conflict)
				}

				// 2. Activate taints
				for _, taint := range isolation.Taint {
					ts.activeTaints[taint]++
				}

				// 3. Remove test from queue
				ts.tests = append(ts.tests[:i], ts.tests[i+1:]...)

				// 4. Return the test (now safe to run)
				return spec
			}
		}

		// No runnable test found, but tests still exist in queue - wait for state change
		ts.cond.Wait()
	}
}

func getConflictGroup(_ *ExtensionTestSpec) string {
	return defaultConflictGroup
}

// hasActiveConflict checks if the spec has any conflicts with currently running tests.
func (ts *testScheduler) hasActiveConflict(spec *ExtensionTestSpec, conflictGroup string) bool {
	for _, conflict := range spec.Resources.Isolation.Conflict {
		if ts.runningConflicts[conflictGroup].Has(conflict) {
			return true
		}
	}
	return false
}

// canTolerateTaints checks if a spec can tolerate all currently active taints.
func (ts *testScheduler) canTolerateTaints(spec *ExtensionTestSpec) bool {
	// If no taints are active, any test can run
	if len(ts.activeTaints) == 0 {
		return true
	}

	// Build a set of tolerations for efficient lookup
	tolerations := sets.New(spec.Resources.Isolation.Toleration...)

	// Check if test tolerates all active taints
	for taint, count := range ts.activeTaints {
		// Skip taints with zero count (should be cleaned up but being defensive)
		if count <= 0 {
			continue
		}

		if !tolerations.Has(taint) {
			return false // Test cannot tolerate this active taint
		}
	}
	return true
}

// MarkTestComplete marks all conflicts and taints of a spec as no longer running/active
// and signals waiting workers that blocked tests may now be runnable.
// This should be called after a test completes execution.
func (ts *testScheduler) MarkTestComplete(spec *ExtensionTestSpec) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if spec == nil {
		ts.cond.Broadcast()
		return
	}

	isolation := &spec.Resources.Isolation
	conflictGroup := getConflictGroup(spec)

	// Clean up conflicts within this group
	if groupConflicts, exists := ts.runningConflicts[conflictGroup]; exists {
		for _, conflict := range isolation.Conflict {
			groupConflicts.Delete(conflict)
		}
	}

	// Clean up taints with reference counting
	for _, taint := range isolation.Taint {
		ts.activeTaints[taint]--
		if ts.activeTaints[taint] <= 0 {
			delete(ts.activeTaints, taint)
		}
	}

	// Signal waiting workers that the state has changed
	// Some blocked tests might now be runnable
	ts.cond.Broadcast()
}
