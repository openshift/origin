package ginkgo

import (
	"context"
	"fmt"
	"io"
	"strings"
	//"sync"
)

// parallelByFileTestQueue runs tests in parallel unless they have
// the `[Serial]` tag on their name or if another test with the
// testExclusion field is currently running. Serial tests are
// defered until all other tests are completed.
type parallelByFileTestQueue struct {
	commandContext *commandContext
}

type TestFunc func(ctx context.Context, test *testCase)

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

	close(remainingParallelTests)
}

// runTestsUntilChannelEmpty reads from the channel to consume tests, run them, and return when the channel is closed.
func runTestsUntilChannelEmpty(ctx context.Context, remainingParallelTests chan *testCase, testSuiteRunner testSuiteRunner) {
	for {
		select {
		// if the context is finished, simply return
		case <-ctx.Done():
			return

		case test, ok := <-remainingParallelTests:
			if !ok { // channel closed, then we're done
				return
			}
			// if the context is finished, simply return
			if ctx.Err() != nil {
				return
			}
			testSuiteRunner.RunOneTest(ctx, test)
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

	serial, parallel := splitTests(tests, isSerialTest)

	// TODO: can combine this using ginkgo's serial label?
	testSuiteRunner.RunMultipleTests(ctx, parallel...)
	testSuiteRunner.RunMultipleTests(ctx, serial...)

	/*
		remainingParallelTests := make(chan *testCase, 100)
		go queueAllTests(remainingParallelTests, parallel)

		var wg sync.WaitGroup
		for i := 0; i < parallelism; i++ {
			wg.Add(1)
			go func(ctx context.Context) {
				defer wg.Done()
				runTestsUntilChannelEmpty(ctx, remainingParallelTests, testSuiteRunner)
			}(ctx)
		}
		wg.Wait()

		for _, test := range serial {
			if ctx.Err() != nil {
				return
			}
			testSuiteRunner.RunOneTest(ctx, test)
		}
	*/
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
