package ginkgo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type testSuiteRunner interface {
	RunOneTest(ctx context.Context, test *testCase)
}

// testRunner contains all the content required to run a test.  It must be threadsafe and must be re-useable
// across multiple parallel RunOneTest invocations.
type testSuiteRunnerImpl struct {
	commandContext        *commandContext
	testOutput            testOutputConfig
	testSuiteProgress     *testSuiteProgress
	maybeAbortOnFailureFn testAbortFunc
}

// RunOneTest runs a test, mutates the testCase with result, and reports the result
func (r *testSuiteRunnerImpl) RunOneTest(ctx context.Context, test *testCase) {
	// this construct is a little odd, but the defer statements that ensure we run at the end have the variables
	// assigned/resolved at the time defer statement is created, which means that pointer must remain consistent.
	// however, we can change the values of the content, so we assign the content lower down to have a cleaner set of
	// the many defers in this function.
	// remember that defers are last-added, first-executed.
	testRunResult := &testRunResultHandle{}

	// if we need to abort, then abort
	defer r.maybeAbortOnFailureFn(testRunResult)

	// record the test happening the monitor
	r.testOutput.monitorRecorder.Record(monitorapi.Condition{
		Level:   monitorapi.Info,
		Locator: monitorapi.E2ETestLocator(test.name),
		Message: "started",
	})
	defer recordTestResultInMonitor(testRunResult, r.testOutput.monitorRecorder)

	// log the results to systemout
	r.testSuiteProgress.LogTestStart(r.testOutput.out, test.name)
	defer r.testSuiteProgress.TestEnded(test.name, testRunResult)
	defer recordTestResultInLogWithoutOverlap(testRunResult, r.testOutput.testOutputLock, r.testOutput.out, r.testOutput.includeSuccessfulOutput)

	testRunResult.testRunResult = r.commandContext.RunTestInNewProcess(ctx, test)
	mutateTestCaseWithResults(test, testRunResult)
}

func mutateTestCaseWithResults(test *testCase, testRunResult *testRunResultHandle) {
	test.start = testRunResult.start
	test.end = testRunResult.end

	// now set the content of the test we know after running the test.
	duration := testRunResult.end.Sub(testRunResult.start).Round(time.Second / 10)
	if duration > time.Minute {
		duration = duration.Round(time.Second)
	}
	test.duration = duration

	test.testOutputBytes = testRunResult.testOutputBytes

	switch testRunResult.testState {
	case TestFlaked:
		test.flake = true
		test.failed = false
		test.skipped = false
		test.success = false
		test.timedOut = false
	case TestSucceeded:
		test.flake = false
		test.failed = false
		test.skipped = false
		test.success = true
		test.timedOut = false
	case TestSkipped:
		test.flake = false
		test.failed = false
		test.skipped = true
		test.success = false
		test.timedOut = false
	case TestFailed:
		test.flake = false
		test.failed = true
		test.skipped = false
		test.success = false
		test.timedOut = false
	case TestFailedTimeout:
		test.flake = false
		test.failed = true
		test.skipped = false
		test.success = false
		test.timedOut = true
	case TestUnknown:
		test.flake = false
		test.failed = true
		test.skipped = false
		test.success = false
		test.timedOut = false
	default:
		panic("unhandled test case state")
	}
}

type commandContext struct {
	env     []string
	timeout time.Duration

	testOutputConfig testOutputConfig
}

type testOutputConfig struct {
	testOutputLock  *sync.Mutex
	out             io.Writer
	monitorRecorder monitor.Recorder

	includeSuccessfulOutput bool
}

type testRunResultHandle struct {
	*testRunResult
}

type testRunResult struct {
	name            string
	start           time.Time
	end             time.Time
	testState       TestState
	testOutputBytes []byte
}

func (r testRunResult) duration() time.Duration {
	duration := r.end.Sub(r.start).Round(time.Second / 10)
	if duration > time.Minute {
		duration = duration.Round(time.Second)
	}
	return duration
}

type TestState string

const (
	TestSucceeded     TestState = "Success"
	TestFailed        TestState = "Failed"
	TestFailedTimeout TestState = "TimedOut"
	TestFlaked        TestState = "Flaked"
	TestSkipped       TestState = "Skipped"
	TestUnknown       TestState = "Unknown"
)

func isTestFailed(testState TestState) bool {
	switch testState {
	case TestSucceeded:
		return false
	case TestFlaked:
		return false
	case TestSkipped:
		return false
	}
	return true
}

// testOutputLock prevents parallel tests from interleaving their output.
func newTestOutputConfig(testOutputLock *sync.Mutex, out io.Writer, monitorRecorder monitor.Recorder, includeSuccessfulOutput bool) testOutputConfig {
	return testOutputConfig{
		testOutputLock:          testOutputLock,
		out:                     out,
		monitorRecorder:         monitorRecorder,
		includeSuccessfulOutput: includeSuccessfulOutput,
	}
}

// construction provided so that if we add anything, we get a compile failure for all callers instead of weird behavior
func newCommandContext(env []string, timeout time.Duration) *commandContext {
	return &commandContext{
		env:     env,
		timeout: timeout,
	}
}

func (c *commandContext) commandString(test *testCase) string {
	buf := &bytes.Buffer{}
	for _, env := range c.env {
		parts := strings.SplitN(env, "=", 2)
		fmt.Fprintf(buf, "%s=%q ", parts[0], parts[1])
	}
	fmt.Fprintf(buf, "%s %s %q", os.Args[0], "run-test", test.name)
	return buf.String()
}

func recordTestResultInLogWithoutOverlap(testRunResult *testRunResultHandle, testOutputLock *sync.Mutex, out io.Writer, includeSuccessfulOutput bool) {
	testOutputLock.Lock()
	defer testOutputLock.Unlock()

	recordTestResultInLog(testRunResult, out, includeSuccessfulOutput)
}

func recordTestResultInLog(testRunResult *testRunResultHandle, out io.Writer, includeSuccessfulOutput bool) {
	// output the status of the test
	switch testRunResult.testState {
	case TestFlaked:
		out.Write(testRunResult.testOutputBytes)
		fmt.Fprintln(out)
		fmt.Fprintf(out, "flaked: (%s) %s %q\n\n", testRunResult.duration(), testRunResult.end.UTC().Format("2006-01-02T15:04:05"), testRunResult.name)
	case TestSucceeded:
		if includeSuccessfulOutput {
			out.Write(testRunResult.testOutputBytes)
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "passed: (%s) %s %q\n\n", testRunResult.duration(), testRunResult.end.UTC().Format("2006-01-02T15:04:05"), testRunResult.name)
	case TestSkipped:
		if includeSuccessfulOutput {
			out.Write(testRunResult.testOutputBytes)
			fmt.Fprintln(out)
		} else {
			message := lastLinesUntil(string(testRunResult.testOutputBytes), 100, "skip [")
			if len(message) > 0 {
				fmt.Fprintln(out, message)
				fmt.Fprintln(out)
			}
		}
		fmt.Fprintf(out, "skipped: (%s) %s %q\n\n", testRunResult.duration(), testRunResult.end.UTC().Format("2006-01-02T15:04:05"), testRunResult.name)
	case TestFailed, TestFailedTimeout:
		out.Write(testRunResult.testOutputBytes)
		fmt.Fprintln(out)
		fmt.Fprintf(out, "failed: (%s) %s %q\n\n", testRunResult.duration(), testRunResult.end.UTC().Format("2006-01-02T15:04:05"), testRunResult.name)
	default:
		out.Write(testRunResult.testOutputBytes)
		fmt.Fprintln(out)
		fmt.Fprintf(out, "UNKNOWN_TEST_STATE=%q: (%s) %s %q\n\n", testRunResult.testState, testRunResult.duration(), testRunResult.end.UTC().Format("2006-01-02T15:04:05"), testRunResult.name)
	}
}

func recordTestResultInMonitor(testRunResult *testRunResultHandle, monitorRecorder monitor.Recorder) {
	eventMessage := "finishedStatus/Unknown reason/Unknown"
	eventLevel := monitorapi.Warning

	switch testRunResult.testState {
	case TestFlaked:
		eventMessage = "finishedStatus/Flaked"
		eventLevel = monitorapi.Error
	case TestSucceeded:
		eventMessage = "finishedStatus/Passed"
		eventLevel = monitorapi.Info
	case TestSkipped:
		eventMessage = "finishedStatus/Skipped"
		eventLevel = monitorapi.Info
	case TestFailed:
		eventMessage = "finishedStatus/Failed"
		eventLevel = monitorapi.Error
	case TestFailedTimeout:
		eventMessage = "finishedStatus/Failed  reason/Timeout"
		eventLevel = monitorapi.Error
	default:
		eventMessage = fmt.Sprintf("finishedStatus/Failed  reason/%s", testRunResult.testState)
		eventLevel = monitorapi.Error
	}

	monitorRecorder.Record(monitorapi.Condition{
		Level:   eventLevel,
		Locator: monitorapi.E2ETestLocator(testRunResult.name),
		Message: eventMessage,
	})
}

// RunTestInNewProcess runs a test case in a different process and returns a result
func (c *commandContext) RunTestInNewProcess(ctx context.Context, test *testCase) *testRunResult {
	ret := &testRunResult{
		name:      test.name,
		testState: TestUnknown,
	}

	// if the test was already marked as skipped, skip it.
	if test.skipped {
		ret.testState = TestSkipped
		return ret
	}

	ret.start = time.Now()
	if test.binary == "" {
		test.binary = os.Args[0]
	}
	if test.nameFromBinary == "" {
		test.nameFromBinary = test.name
	}
	command := exec.Command(test.binary, "run-test", test.nameFromBinary)
	command.Env = append(os.Environ(), c.env...)

	timeout := c.timeout
	if test.testTimeout != 0 {
		timeout = test.testTimeout
	}

	testOutputBytes, err := runWithTimeout(ctx, command, timeout)
	ret.end = time.Now()

	ret.testOutputBytes = testOutputBytes
	if err == nil {
		ret.testState = TestSucceeded
		return ret
	}

	if ctx.Err() != nil {
		ret.testState = TestSkipped
		return ret
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		switch exitErr.ProcessState.Sys().(syscall.WaitStatus).ExitStatus() {
		case 1:
			// failed
			ret.testState = TestFailed
		case 2:
			// timeout (ABRT is an exit code 2)
			ret.testState = TestFailedTimeout
		case 3:
			// skipped
			ret.testState = TestSkipped
		case 4:
			// flaky, do not retry
			ret.testState = TestFlaked
		default:
			ret.testState = TestUnknown
		}
		return ret
	}

	ret.testState = TestFailed
	return ret
}

func runWithTimeout(ctx context.Context, c *exec.Cmd, timeout time.Duration) ([]byte, error) {
	if timeout > 0 {
		go func() {
			select {
			// interrupt tests after timeout, and abort if they don't complete quick enough
			case <-time.After(timeout):
				if c.Process != nil {
					c.Process.Signal(syscall.SIGINT)
				}
				// if the process appears to be hung a significant amount of time after the timeout
				// send an ABRT so we get a stack dump
				select {
				case <-time.After(time.Minute):
					if c.Process != nil {
						c.Process.Signal(syscall.SIGABRT)
					}
				}
			case <-ctx.Done():
				if c.Process != nil {
					c.Process.Signal(syscall.SIGINT)
				}
			}

		}()
	}
	return c.CombinedOutput()
}
