package ginkgo

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

// parallelByFileTestQueue runs tests in parallel unless they have
// the `[Serial]` tag on their name or if another test with the
// testExclusion field is currently running. Serial tests are
// defered until all other tests are completed.
type parallelByFileTestQueue struct {
	commandContext *commandContext
}

// getTestConflictGroup returns the conflict group for a test.
// Conflicts are only checked within the same conflict group.
func getTestConflictGroup(test *testCase) string {
	return "default"
}

// TestScheduler defines the interface for test scheduling
// Different implementations can provide various scheduling strategies
type TestScheduler interface {
	// GetNextTestToRun returns the next test that can run, or nil if none available
	// When a test is returned, it is marked as running atomically
	GetNextTestToRun() *testCase

	// MarkTestComplete marks a test as complete, cleaning up its conflicts and taints
	// This may unblock other tests that were waiting
	MarkTestComplete(test *testCase)

	// IsEmpty returns true if all tests have been scheduled
	IsEmpty() bool

	// WaitForStateChange blocks until scheduler state changes (a test completes)
	// Callers should hold no locks when calling this method
	WaitForStateChange()
}

// testScheduler manages test scheduling based on conflicts, taints, and tolerations
// It maintains an ordered queue of tests and provides thread-safe scheduling operations
type testScheduler struct {
	mu               sync.Mutex
	cond             *sync.Cond                  // condition variable to signal when tests complete
	tests            []*testCase                 // ordered queue of tests to execute
	runningConflicts map[string]sets.Set[string] // tracks which conflicts are running per group: group -> set of conflicts
	activeTaints     map[string]int              // tracks how many tests are currently applying each taint
}

// newTestScheduler creates a test scheduler. Potentially this can order the
// tests in any order and schedule tests based on resulted order.
func newTestScheduler(tests []*testCase) TestScheduler {
	ts := &testScheduler{
		tests:            tests,
		runningConflicts: make(map[string]sets.Set[string]),
		activeTaints:     make(map[string]int),
	}
	ts.cond = sync.NewCond(&ts.mu)
	return ts
}

// GetNextTestToRun scans the queue from the beginning and returns the first test that can run
// Returns nil if no runnable test is found or if the queue is empty
// When a test is returned, it is removed from the queue AND marked as running atomically
func (ts *testScheduler) GetNextTestToRun() *testCase {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Scan from beginning to find first runnable test
	for i, test := range ts.tests {
		conflictGroup := getTestConflictGroup(test)

		// Ensure the conflict group set exists
		if ts.runningConflicts[conflictGroup] == nil {
			ts.runningConflicts[conflictGroup] = sets.New[string]()
		}

		// Check if any of the test's conflicts are currently running within its group
		hasConflict := false
		for _, conflict := range test.isolation.Conflict {
			if ts.runningConflicts[conflictGroup].Has(conflict) {
				hasConflict = true
				break
			}
		}

		// Check if test can tolerate all currently active taints
		canTolerate := ts.canTolerateTaints(test)

		if !hasConflict && canTolerate {
			// Found a runnable test - ATOMICALLY:
			// 1. Mark conflicts as running
			for _, conflict := range test.isolation.Conflict {
				ts.runningConflicts[conflictGroup].Insert(conflict)
			}

			// 2. Activate taints
			for _, taint := range test.isolation.Taint {
				ts.activeTaints[taint]++
			}

			// 3. Remove test from queue
			ts.tests = append(ts.tests[:i], ts.tests[i+1:]...)

			// 4. Return the test (now safe to run)
			return test
		}
	}

	// No runnable test found
	return nil
}

// IsEmpty checks if the queue is empty
func (ts *testScheduler) IsEmpty() bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.tests) == 0
}

// size returns the current size of the queue
func (ts *testScheduler) size() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.tests)
}

// canTolerateTaints checks if a test can tolerate all currently active taints
func (ts *testScheduler) canTolerateTaints(test *testCase) bool {
	// Check if test tolerates all active taints
	for taint, count := range ts.activeTaints {
		// Skip taints with zero count (should be cleaned up but being defensive)
		if count <= 0 {
			continue
		}

		tolerated := false
		for _, toleration := range test.isolation.Toleration {
			if toleration == taint {
				tolerated = true
				break
			}
		}
		if !tolerated {
			return false // Test cannot tolerate this active taint
		}
	}
	return true
}

// MarkTestComplete marks all conflicts and taints of a test as no longer running/active
// This should be called after a test completes execution
func (ts *testScheduler) MarkTestComplete(test *testCase) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Get the conflict group for this test
	conflictGroup := getTestConflictGroup(test)

	// Clean up conflicts within this group
	if groupConflicts, exists := ts.runningConflicts[conflictGroup]; exists {
		for _, conflict := range test.isolation.Conflict {
			groupConflicts.Delete(conflict)
		}
	}

	// Clean up taints with reference counting
	for _, taint := range test.isolation.Taint {
		ts.activeTaints[taint]--
		if ts.activeTaints[taint] <= 0 {
			delete(ts.activeTaints, taint)
		}
	}

	// Signal waiting workers that the state has changed
	// Some blocked tests might now be runnable
	ts.cond.Broadcast()
}

// WaitForStateChange blocks until the scheduler state changes (a test completes)
// This implements the TestScheduler interface
func (ts *testScheduler) WaitForStateChange() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.cond.Wait()
}

func newParallelTestQueue(commandContext *commandContext) *parallelByFileTestQueue {
	return &parallelByFileTestQueue{
		commandContext: commandContext,
	}
}

// OutputCommand prints to stdout what would have been executed.
func (q *parallelByFileTestQueue) OutputCommands(ctx context.Context, tests []*testCase, out io.Writer) {
	// for some reason we split the serial and parallel when printing the command
	serial, parallel := splitTests(tests, func(t *testCase) bool { return strings.Contains(t.name, "[Serial]") })

	for _, curr := range parallel {
		commandString := q.commandContext.commandString(curr)
		fmt.Fprintln(out, commandString)
	}
	for _, curr := range serial {
		commandString := q.commandContext.commandString(curr)
		fmt.Fprintln(out, commandString)
	}
}

// testAbortFunc can be called to abort the running tests.
type testAbortFunc func(testRunResult *testRunResultHandle)

func neverAbort(_ *testRunResultHandle) {}

// abortOnFailure returns an abort function and a context that will be cancelled on the abort.
func abortOnFailure(parentContext context.Context) (testAbortFunc, context.Context) {
	testCtx, cancelFn := context.WithCancel(parentContext)
	return func(testRunResult *testRunResultHandle) {
		if isTestFailed(testRunResult.testState) {
			cancelFn()
		}
	}, testCtx
}

// runTestsUntilDone continuously polls the scheduler for runnable tests
// Workers try to get the next runnable test in order, run it, mark complete, and repeat
// Returns when all tests are complete or context is cancelled
func runTestsUntilDone(ctx context.Context, scheduler TestScheduler, testSuiteRunner testSuiteRunner) {
	for {
		// Check context first
		if ctx.Err() != nil {
			return
		}

		// Try to get next runnable test (maintains order)
		test := scheduler.GetNextTestToRun()

		if test == nil {
			// No runnable test found
			if scheduler.IsEmpty() {
				// All tests have been scheduled, we're done
				return
			}

			// Queue has tests but none can run right now
			// Wait for scheduler state to change (a test completes)
			scheduler.WaitForStateChange()
			continue
		}

		// Found a runnable test - execute it
		testSuiteRunner.RunOneTest(ctx, test)

		// Mark test as complete (clean up conflicts and taints)
		// This will also signal waiting workers
		scheduler.MarkTestComplete(test)
	}
}

// tests are currently being mutated during the run process.
func (q *parallelByFileTestQueue) Execute(ctx context.Context, tests []*testCase, parallelism int, testOutput testOutputConfig, maybeAbortOnFailureFn testAbortFunc) {
	testSuiteProgress := newTestSuiteProgress(len(tests))
	testSuiteRunner := &testSuiteRunnerImpl{
		commandContext:        q.commandContext,
		testOutput:            testOutput,
		testSuiteProgress:     testSuiteProgress,
		maybeAbortOnFailureFn: maybeAbortOnFailureFn,
	}

	execute(ctx, testSuiteRunner, tests, parallelism)
}

// execute is a convenience for unit testing
func execute(ctx context.Context, testSuiteRunner testSuiteRunner, tests []*testCase, parallelism int) {
	if ctx.Err() != nil {
		return
	}

	// Split tests into two categories: serial and parallel (including isolated)
	serial, parallel := splitTests(tests, isSerialTest)

	if len(parallel) > 0 {
		// Create test scheduler with all parallel tests
		// TestScheduler encapsulates the queue and scheduling logic
		var scheduler TestScheduler = newTestScheduler(parallel)

		var wg sync.WaitGroup

		// Run all non-serial tests with conflict-aware workers
		// Each worker polls the scheduler for the next runnable test in order
		for i := 0; i < parallelism; i++ {
			wg.Add(1)
			go func(ctx context.Context) {
				defer wg.Done()
				runTestsUntilDone(ctx, scheduler, testSuiteRunner)
			}(ctx)
		}

		wg.Wait()
	}

	// Run serial tests sequentially at the end
	for _, test := range serial {
		if ctx.Err() != nil {
			return
		}
		testSuiteRunner.RunOneTest(ctx, test)
	}
}

func isSerialTest(test *testCase) bool {
	if strings.Contains(test.name, "[Serial]") {
		return true
	}

	return false
}

func copyTests(tests []*testCase) []*testCase {
	copied := make([]*testCase, 0, len(tests))
	for _, t := range tests {
		c := *t
		copied = append(copied, &c)
	}
	return copied
}

func splitTests(tests []*testCase, fn func(*testCase) bool) (a, b []*testCase) {
	for _, t := range tests {
		if fn(t) {
			a = append(a, t)
		} else {
			b = append(b, t)
		}
	}
	return a, b
}

