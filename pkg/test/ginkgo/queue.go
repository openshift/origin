package ginkgo

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// parallelByFileTestQueue runs tests in parallel unless they have
// the `[Serial]` tag on their name or if another test with the
// testExclusion field is currently running. Serial tests are
// defered until all other tests are completed.
type parallelByFileTestQueue struct {
	commandContext *commandContext
}

type TestFunc func(ctx context.Context, test *testCase)

// getTestRunningInstance returns the exec instance identifier for a test.
func getTestRunningInstance(test *testCase) string {
	// TODO: Implement actual instance assignment
	return "instance-1"
}

// getTestBucket returns the bucket identifier for a test.
func getTestBucket(test *testCase) string {
	// TODO: Implement actual bucket assignment
	return "bucket-a"
}

// getTestConflictGroup returns the conflict group for a test.
// Conflicts are only checked within the same conflict group.
// The group is determined by the test's isolation mode:
//   - "instance": conflicts scoped to the running instance
//   - "bucket": conflicts scoped to a test bucket
//   - "exec" or empty: conflicts scoped to "default" group
func getTestConflictGroup(test *testCase) string {
	mode := test.isolation.Mode

	switch mode {
	case "instance":
		return getTestRunningInstance(test)
	case "bucket":
		return getTestBucket(test)
	case "exec", "":
		return "default"
	default:
		// Unknown mode, fall back to default
		return "default"
	}
}

// testScheduler manages test scheduling based on conflicts, taints, and tolerations
type testScheduler struct {
	mu               sync.Mutex
	runningConflicts map[string]map[string]bool // tracks which conflicts are running per group: group -> conflict -> bool
	activeTaints     map[string]int             // tracks how many tests are currently applying each taint
}

func newTestScheduler() *testScheduler {
	return &testScheduler{
		runningConflicts: make(map[string]map[string]bool),
		activeTaints:     make(map[string]int),
	}
}

// decrementAndCloseIfDone decrements the pending test counter and closes the channel if all tests are done
func decrementAndCloseIfDone(pendingTestCount *int64, remainingTests chan *testCase) {
	if atomic.AddInt64(pendingTestCount, -1) == 0 {
		// Use defer and recover to handle potential double-close
		defer func() {
			if r := recover(); r != nil {
				// Channel already closed, ignore
			}
		}()
		close(remainingTests)
	}
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
		for _, toleration := range test.toleration {
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

// tryRunTest atomically checks if a test can run and executes it synchronously if possible
// Returns true if the test was executed, false if there are conflicts or taint/toleration issues
// The function blocks until the test completes before returning
func (ts *testScheduler) tryRunTest(ctx context.Context, test *testCase, testSuiteRunner testSuiteRunner, pendingTestCount *int64, remainingTests chan *testCase) bool {
	ts.mu.Lock()

	// Get the conflict group for this test
	conflictGroup := getTestConflictGroup(test)

	// Ensure the conflict group map exists
	if ts.runningConflicts[conflictGroup] == nil {
		ts.runningConflicts[conflictGroup] = make(map[string]bool)
	}

	// Check if any of the test's conflicts are currently running within its group
	for _, conflict := range test.isolation.Conflict {
		if ts.runningConflicts[conflictGroup][conflict] {
			ts.mu.Unlock()
			return false // Cannot run due to conflicts within the same group
		}
	}

	// Check if test can tolerate all currently active taints
	if !ts.canTolerateTaints(test) {
		ts.mu.Unlock()
		return false // Cannot run due to taint/toleration mismatch
	}

	// All checks passed - mark conflicts as running within this group and activate taints
	for _, conflict := range test.isolation.Conflict {
		ts.runningConflicts[conflictGroup][conflict] = true
	}

	for _, taint := range test.taint {
		ts.activeTaints[taint]++
	}

	// Release lock before running test (since test execution can take a while)
	ts.mu.Unlock()

	// Run the test synchronously
	testSuiteRunner.RunOneTest(ctx, test)

	// Clean up conflicts, taints, and decrement counter
	ts.markTestCompleted(test)
	decrementAndCloseIfDone(pendingTestCount, remainingTests)

	return true
}

// markTestCompleted marks all conflicts and taints of a test as no longer running/active
func (ts *testScheduler) markTestCompleted(test *testCase) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Get the conflict group for this test
	conflictGroup := getTestConflictGroup(test)

	// Clean up conflicts within this group
	if groupConflicts, exists := ts.runningConflicts[conflictGroup]; exists {
		for _, conflict := range test.isolation.Conflict {
			delete(groupConflicts, conflict)
		}
	}

	// Clean up taints with reference counting
	for _, taint := range test.taint {
		ts.activeTaints[taint]--
		if ts.activeTaints[taint] <= 0 {
			delete(ts.activeTaints, taint)
		}
	}
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

// queueAllTests writes all the tests to the channel and closes it when all are finished
// even with buffering, this can take a while since we don't infinitely buffer.
func queueAllTests(remainingParallelTests chan *testCase, tests []*testCase) {
	for i := range tests {
		curr := tests[i]
		remainingParallelTests <- curr
	}
	// Don't close the channel here - workers may need to put tests back
}

// runTestsUntilChannelEmpty reads from the channel to consume tests, run them, and return when no more tests are pending.
// This version is conflict-aware and can handle both parallel and isolated tests.
// If a test can't run due to conflicts, it's put back at the end of the queue.
// Uses atomic counter to coordinate when all work is truly done.
func runTestsUntilChannelEmpty(ctx context.Context, remainingTests chan *testCase, testSuiteRunner testSuiteRunner, scheduler *testScheduler, pendingTestCount *int64) {
	for {
		select {
		// if the context is finished, simply return
		case <-ctx.Done():
			return

		case test, ok := <-remainingTests:
			if !ok { // channel closed, we're done
				return
			}
			// if the context is finished, simply return
			if ctx.Err() != nil {
				return
			}

			// Try to run the test atomically (handles both parallel and isolated tests)
			if !scheduler.tryRunTest(ctx, test, testSuiteRunner, pendingTestCount, remainingTests) {
				// Can't run now due to conflicts, put back at end of queue after a short delay
				go func(t *testCase) {
					// Small delay to avoid busy spinning when many tests are conflicting
					select {
					case <-time.After(10 * time.Millisecond):
					case <-ctx.Done():
						// Context cancelled, decrement pending count since test won't complete
						decrementAndCloseIfDone(pendingTestCount, remainingTests)
						return
					}

					select {
					case remainingTests <- t:
						// Successfully put back in queue
					case <-ctx.Done():
						// Context cancelled, decrement pending count since test won't complete
						decrementAndCloseIfDone(pendingTestCount, remainingTests)
					}
				}(test)
			}
		}
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
		// Create test scheduler for managing test execution with conflicts and taints
		scheduler := newTestScheduler()

		// Start all non-serial tests in a unified channel
		remainingTests := make(chan *testCase, 100)

		// Use atomic counter to track pending tests for proper channel closure coordination
		var pendingTestCount int64 = int64(len(parallel))

		go queueAllTests(remainingTests, parallel)

		var wg sync.WaitGroup

		// Run all non-serial tests with conflict-aware workers
		for i := 0; i < parallelism; i++ {
			wg.Add(1)
			go func(ctx context.Context) {
				defer wg.Done()
				runTestsUntilChannelEmpty(ctx, remainingTests, testSuiteRunner, scheduler, &pendingTestCount)
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
