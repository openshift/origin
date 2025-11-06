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

// conflictTracker manages test conflicts to ensure only one test per conflict group runs at a time
type conflictTracker struct {
	mu               sync.Mutex
	runningConflicts map[string]bool // tracks which conflict groups are currently running
}

func newConflictTracker() *conflictTracker {
	return &conflictTracker{
		runningConflicts: make(map[string]bool),
	}
}

// decrementAndCloseIfDone decrements the pending test counter and closes the channel if all tests are done
func decrementAndCloseIfDone(pendingTestCount *int64, remainingTests chan *testCase) {
	if atomic.AddInt64(pendingTestCount, -1) == 0 {
		close(remainingTests)
	}
}

// tryRunTest atomically checks if a test can run and executes it synchronously if possible
// Returns true if the test was executed, false if there are conflicts
// The function blocks until the test completes before returning
func (ct *conflictTracker) tryRunTest(ctx context.Context, test *testCase, testSuiteRunner testSuiteRunner, pendingTestCount *int64, remainingTests chan *testCase) bool {
	// For tests with no isolation conflicts, run directly
	if len(test.isolation.Conflict) == 0 {
		testSuiteRunner.RunOneTest(ctx, test)
		// Decrement pending counter when test completes
		decrementAndCloseIfDone(pendingTestCount, remainingTests)
		return true
	}

	// For isolated tests, check conflicts atomically
	ct.mu.Lock()

	// Check if any of the test's conflict groups are currently running
	for _, conflict := range test.isolation.Conflict {
		if ct.runningConflicts[conflict] {
			ct.mu.Unlock()
			return false // Cannot run due to conflicts
		}
	}

	// No conflicts found, mark all conflict groups as running
	for _, conflict := range test.isolation.Conflict {
		ct.runningConflicts[conflict] = true
	}

	ct.mu.Unlock()

	// Run the test synchronously with conflict cleanup
	testSuiteRunner.RunOneTest(ctx, test)

	// Clean up conflicts and decrement counter
	ct.markTestCompleted(test)
	decrementAndCloseIfDone(pendingTestCount, remainingTests)

	return true
}

// markTestCompleted marks all conflict groups of a test as no longer running
func (ct *conflictTracker) markTestCompleted(test *testCase) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	for _, conflict := range test.isolation.Conflict {
		delete(ct.runningConflicts, conflict)
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
func runTestsUntilChannelEmpty(ctx context.Context, remainingTests chan *testCase, testSuiteRunner testSuiteRunner, conflictTracker *conflictTracker, pendingTestCount *int64) {
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
			if !conflictTracker.tryRunTest(ctx, test, testSuiteRunner, pendingTestCount, remainingTests) {
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
		// Create conflict tracker for isolated tests
		conflictTracker := newConflictTracker()

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
				runTestsUntilChannelEmpty(ctx, remainingTests, testSuiteRunner, conflictTracker, &pendingTestCount)
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
