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
// Conflict group is a concept designed to support future functionality
// like mode defined in Isolation. As of now, all tests belong to the
// default group and behave like the "exec" mode.
func getTestConflictGroup(test *testCase) string {
	return "default"
}

// TestScheduler defines the interface for test scheduling
// Different implementations can provide various scheduling strategies
type TestScheduler interface {
	// GetNextTestToRun blocks until a test is available, then returns it.
	// Returns nil when all tests have been distributed (queue is empty) or context is cancelled.
	// When a test is returned, it is atomically removed from queue and marked as running.
	// This method can be safely called from multiple goroutines concurrently.
	GetNextTestToRun(ctx context.Context) *testCase

	// MarkTestComplete marks a test as complete, cleaning up its conflicts and taints.
	// This may unblock other tests that were waiting.
	// This method can be safely called from multiple goroutines concurrently.
	MarkTestComplete(test *testCase)
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

// GetNextTestToRun blocks until a test is available to run, or returns nil if all tests have been distributed
// or the context is cancelled. It continuously scans the queue and waits for state changes when no tests are runnable.
// When a test is returned, it is atomically removed from queue and marked as running.
func (ts *testScheduler) GetNextTestToRun(ctx context.Context) *testCase {
	ts.mu.Lock()
	defer ts.mu.Unlock()

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
		for i, test := range ts.tests {
			conflictGroup := getTestConflictGroup(test)

			// Ensure the conflict group set exists
			if ts.runningConflicts[conflictGroup] == nil {
				ts.runningConflicts[conflictGroup] = sets.New[string]()
			}

			// Check if any of the test's conflicts are currently running within its group
			hasConflict := false
			if test.spec != nil {
				for _, conflict := range test.spec.Resources.Isolation.Conflict {
					if ts.runningConflicts[conflictGroup].Has(conflict) {
						hasConflict = true
						break
					}
				}
			}

			// Check if test can tolerate all currently active taints
			canTolerate := ts.canTolerateTaints(test)

			if !hasConflict && canTolerate {
				// Found a runnable test - ATOMICALLY:
				// 1. Mark conflicts as running
				if test.spec != nil {
					for _, conflict := range test.spec.Resources.Isolation.Conflict {
						ts.runningConflicts[conflictGroup].Insert(conflict)
					}

					// 2. Activate taints
					for _, taint := range test.spec.Resources.Isolation.Taint {
						ts.activeTaints[taint]++
					}
				}

				// 3. Remove test from queue
				ts.tests = append(ts.tests[:i], ts.tests[i+1:]...)

				// 4. Return the test (now safe to run)
				return test
			}
		}

		// No runnable test found, but tests still exist in queue - wait for state change
		ts.cond.Wait()
	}
}

// canTolerateTaints checks if a test can tolerate all currently active taints
func (ts *testScheduler) canTolerateTaints(test *testCase) bool {
	// If test has no spec, it has no toleration requirements (can run with any taints)
	if test.spec == nil {
		return len(ts.activeTaints) == 0 // Can only run if no taints are active
	}

	// Check if test tolerates all active taints
	for taint, count := range ts.activeTaints {
		// Skip taints with zero count (should be cleaned up but being defensive)
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
			return false // Test cannot tolerate this active taint
		}
	}
	return true
}

// MarkTestComplete marks all conflicts and taints of a test as no longer running/active
// and signals waiting workers that blocked tests may now be runnable
// This should be called after a test completes execution
func (ts *testScheduler) MarkTestComplete(test *testCase) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// If test has no spec, there's nothing to clean up
	if test.spec != nil {
		// Get the conflict group for this test
		conflictGroup := getTestConflictGroup(test)

		// Clean up conflicts within this group
		if groupConflicts, exists := ts.runningConflicts[conflictGroup]; exists {
			for _, conflict := range test.spec.Resources.Isolation.Conflict {
				groupConflicts.Delete(conflict)
			}
		}

		// Clean up taints with reference counting
		for _, taint := range test.spec.Resources.Isolation.Taint {
			ts.activeTaints[taint]--
			if ts.activeTaints[taint] <= 0 {
				delete(ts.activeTaints, taint)
			}
		}
	}

	// Signal waiting workers that the state has changed
	// Some blocked tests might now be runnable
	ts.cond.Broadcast()
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

// runTestsUntilDone continuously gets tests from the scheduler, runs them, and marks them complete.
// GetNextTestToRun() blocks internally when no tests are runnable and returns nil when all tests are distributed
// or context is cancelled. Returns when there are no more tests to take from the queue or context is cancelled.
func runTestsUntilDone(ctx context.Context, scheduler TestScheduler, testSuiteRunner testSuiteRunner) {
	for {
		// Get next test - this blocks until a test is available, queue is empty, or context is cancelled
		test := scheduler.GetNextTestToRun(ctx)

		if test == nil {
			// No more tests to take from queue or context cancelled
			return
		}

		// Run the test
		testSuiteRunner.RunOneTest(ctx, test)

		// Mark test as complete (clean up conflicts/taints and signal waiting workers)
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
