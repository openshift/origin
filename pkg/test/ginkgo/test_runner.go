package ginkgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/openshift/origin/pkg/test/extensions"

	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
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

	// record the test happening with the monitor
	r.testOutput.monitorRecorder.AddIntervals(monitorapi.NewInterval(monitorapi.SourceE2ETest, monitorapi.Info).
		Locator(monitorapi.NewLocator().E2ETest(test.name)).
		Message(monitorapi.NewMessage().HumanMessage("started").Reason(monitorapi.E2ETestStarted)).BuildNow())

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

	test.extensionTestResult = testRunResult.extensionTestResult

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
	monitorRecorder monitorapi.Recorder

	includeSuccessfulOutput bool
}

type testRunResultHandle struct {
	*testRunResult
}

type testRunResult struct {
	name                string
	start               time.Time
	end                 time.Time
	testState           TestState
	testOutputBytes     []byte
	extensionTestResult *extensions.ExtensionTestResult
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
func newTestOutputConfig(testOutputLock *sync.Mutex, out io.Writer, monitorRecorder monitorapi.Recorder, includeSuccessfulOutput bool) testOutputConfig {
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
	envs := updateEnvVars(c.env)
	for _, env := range envs {
		parts := strings.SplitN(env, "=", 2)
		fmt.Fprintf(buf, "%s=%q ", parts[0], parts[1])
	}

	testName := test.rawName
	if testName == "" {
		testName = test.name
	}

	fmt.Fprintf(buf, "%s %s %q", os.Args[0], "run-test", testName)
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

func recordTestResultInMonitor(testRunResult *testRunResultHandle, monitorRecorder monitorapi.Recorder) {
	eventLevel := monitorapi.Warning

	msg := monitorapi.NewMessage().HumanMessage("e2e test finished")

	switch testRunResult.testState {
	case TestFlaked:
		eventLevel = monitorapi.Error
		msg = msg.WithAnnotation(monitorapi.AnnotationStatus, "Flaked")
	case TestSucceeded:
		eventLevel = monitorapi.Info
		msg = msg.WithAnnotation(monitorapi.AnnotationStatus, "Passed")
	case TestSkipped:
		eventLevel = monitorapi.Info
		msg = msg.WithAnnotation(monitorapi.AnnotationStatus, "Skipped")
	case TestFailed:
		eventLevel = monitorapi.Error
		msg = msg.WithAnnotation(monitorapi.AnnotationStatus, "Failed")
	case TestFailedTimeout:
		eventLevel = monitorapi.Error
		msg = msg.WithAnnotation(monitorapi.AnnotationStatus, "Failed").Reason(monitorapi.Timeout)
	default:
		msg.HumanMessagef("with unexpected state: %s", testRunResult.testState)
		eventLevel = monitorapi.Error
		msg = msg.WithAnnotation(monitorapi.AnnotationStatus, "Unknown")
	}

	// Record an interval indicating that the test finished. Another interval will be created that
	// links the start/stop intervals and has the duration for the test run in e2etest.go.
	monitorRecorder.AddIntervals(monitorapi.NewInterval(monitorapi.SourceE2ETest, eventLevel).
		Locator(monitorapi.NewLocator().E2ETest(testRunResult.name)).
		Message(msg.HumanMessage("finished").Reason(monitorapi.E2ETestFinished)).BuildNow())
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
	testEnv := append(os.Environ(), updateEnvVars(c.env)...)

	if test.binary != nil {
		results := test.binary.RunTests(ctx, c.timeout, testEnv, test.name)
		if len(results) != 1 {
			fmt.Fprintf(os.Stderr, "warning: expected 1 result from external binary; received %d", len(results))
		}
		switch results[0].Result {
		case extensions.ResultFailed:
			ret.testState = TestFailed
			ret.testOutputBytes = []byte(fmt.Sprintf("%s\n%s", results[0].Output, results[0].Error))
		case extensions.ResultPassed:
			ret.testState = TestSucceeded
		case extensions.ResultSkipped:
			ret.testState = TestSkipped
			ret.testOutputBytes = []byte(results[0].Output)
		}
		ret.start = extensions.Time(results[0].StartTime)
		ret.end = extensions.Time(results[0].EndTime)
		ret.extensionTestResult = results[0]
		return ret
	}

	testName := test.rawName
	if testName == "" {
		testName = test.name
	}

	command := exec.Command(os.Args[0], "run-test", testName)
	command.Env = testEnv

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

func updateEnvVars(envs []string) []string {
	result := []string{}
	for _, env := range envs {
		if !strings.HasPrefix(env, "TEST_PROVIDER") {
			result = append(result, env)
		}
	}
	// copied from provider.go
	// TODO: add error handling, and maybe turn this into sharable helper?
	config := &clusterdiscovery.ClusterConfiguration{}
	clientConfig, _ := framework.LoadConfig(true)
	clusterState, _ := clusterdiscovery.DiscoverClusterState(clientConfig)
	if clusterState != nil {
		config, _ = clusterdiscovery.LoadConfig(clusterState)
	}
	if len(config.ProviderName) == 0 {
		config.ProviderName = "skeleton"
	}
	provider, _ := json.Marshal(config)
	result = append(result, fmt.Sprintf("TEST_PROVIDER=%s", provider))
	// TODO: do we need to inject KUBECONFIG?
	// result = append(result, "KUBECONFIG=%s", )
	return result
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
