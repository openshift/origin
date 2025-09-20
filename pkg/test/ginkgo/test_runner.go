package ginkgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"

	"github.com/openshift/origin/pkg/test/extensions"

	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type testSuiteRunner interface {
	RunOneTest(ctx context.Context, test *testCase)
	RunMultipleTests(ctx context.Context, test ...*testCase)
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

func (r *testSuiteRunnerImpl) RunMultipleTests(ctx context.Context, tests ...*testCase) {
	// this construct is a little odd, but the defer statements that ensure we run at the end have the variables
	// assigned/resolved at the time defer statement is created, which means that pointer must remain consistent.
	// however, we can change the values of the content, so we assign the content lower down to have a cleaner set of
	// the many defers in this function.
	// remember that defers are last-added, first-executed.
	// testRunResults := []testRunResultHandle{}

	for _, test := range tests {
		// record the test happening with the monitor
		r.testOutput.monitorRecorder.AddIntervals(monitorapi.NewInterval(monitorapi.SourceE2ETest, monitorapi.Info).
			Locator(monitorapi.NewLocator().E2ETest(test.name)).
			Message(monitorapi.NewMessage().HumanMessage("started").Reason(monitorapi.E2ETestStarted)).BuildNow())

		r.testSuiteProgress.LogTestStart(r.testOutput.out, test.name)
	}

	// defer recordTestResultInMonitor(testRunResult, r.testOutput.monitorRecorder)

	// log the results to systemout
	// r.testSuiteProgress.LogTestStart(r.testOutput.out, test.name)
	// defer r.testSuiteProgress.TestEnded(test.name, testRunResult)
	// defer recordTestResultInLogWithoutOverlap(testRunResult, r.testOutput.testOutputLock, r.testOutput.out, r.testOutput.includeSuccessfulOutput)

	testRunResults := r.commandContext.RunTestsInNewProcess(ctx, tests...)

	for _, test := range tests {
		ind := slices.IndexFunc(testRunResults, func(e testRunResult) bool {
			return e.name == test.name
		})

		// TODO: no known result case?
		if ind == -1 {
			continue
		}

		res := &testRunResultHandle{&testRunResults[ind]}
		mutateTestCaseWithResults(test, res)

		r.testSuiteProgress.TestEnded(test.name, res)
		recordTestResultInLogWithoutOverlap(res, r.testOutput.testOutputLock, r.testOutput.out, r.testOutput.includeSuccessfulOutput)
	}
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
	if testRunResult == nil {
		fmt.Fprintln(os.Stderr, "testRunResult is nil")
		return
	}

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
	if testRunResult == nil {
		fmt.Fprintln(os.Stderr, "testRunResult is nil")
		return
	}

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

	// Everything's been migrated to OTE, including origin itself, test spec must have a binary set
	if test.binary == nil {
		ret.testState = TestFailed
		ret.testOutputBytes = []byte("test has no binary configured; this should not be possible")
	}

	timeout := c.timeout
	if test.testTimeout > 0 {
		timeout = test.testTimeout
	}

	results := test.binary.RunTests(ctx, timeout, testEnv, test.name)
	if len(results) != 1 {
		fmt.Fprintf(os.Stderr, "warning: expected 1 result from external binary; received %d", len(results))
	}
	if len(results) == 0 {
		ret.testState = TestFailed
		ret.testOutputBytes = []byte("no results from external binary")
		return ret
	}

	switch results[0].Result {
	case extensiontests.ResultFailed:
		ret.testState = TestFailed
		ret.testOutputBytes = []byte(fmt.Sprintf("%s\n%s", results[0].Output, results[0].Error))
	case extensiontests.ResultPassed:
		ret.testState = TestSucceeded
	case extensiontests.ResultSkipped:
		ret.testState = TestSkipped
		ret.testOutputBytes = []byte(results[0].Output)
	}
	ret.start = extensions.Time(results[0].StartTime)
	ret.end = extensions.Time(results[0].EndTime)
	ret.extensionTestResult = results[0]
	return ret
}

func (c *commandContext) RunTestsInNewProcess(ctx context.Context, tests ...*testCase) []testRunResult {
	ret := []testRunResult{}

	testEnv := append(os.Environ(), updateEnvVars(c.env)...)

	testsByBinary := map[*extensions.TestBinary][]*testCase{}

	for _, test := range tests {
		testsByBinary[test.binary] = append(testsByBinary[test.binary], test)
	}

	for _, testNoBinary := range testsByBinary[nil] {
		ret = append(ret, testRunResult{
			name:            testNoBinary.name,
			testState:       TestFailed,
			testOutputBytes: []byte("test has no binary configured; this should not be possible"),
			start:           time.Now(),
		})
	}

	for binary, tests := range testsByBinary {
		// handled earlier
		if binary == nil {
			continue
		}

		testNames := []string{}

		for _, test := range tests {
			if test.skipped {
				ret = append(ret, testRunResult{
					name:      test.name,
					testState: TestSkipped,
					start:     time.Now(),
				})
				continue
			}

			testNames = append(testNames, test.name)
		}

		results := binary.RunTests(ctx, c.timeout, testEnv, testNames...)

		ret = append(ret, testRunResultsForBinaryResults(results...)...)
	}

	return ret
}

func testRunResultsForBinaryResults(results ...*extensions.ExtensionTestResult) []testRunResult {
	ret := []testRunResult{}

	for _, result := range results {
		converted := testRunResult{
			name: result.Name,
		}
		switch result.Result {
		case extensiontests.ResultFailed:
			converted.testState = TestFailed
			converted.testOutputBytes = []byte(fmt.Sprintf("%s\n%s", result.Output, result.Error))
		case extensiontests.ResultPassed:
			converted.testState = TestSucceeded
		case extensiontests.ResultSkipped:
			converted.testState = TestSkipped
			converted.testOutputBytes = []byte(result.Output)
		}
		converted.start = extensions.Time(result.StartTime)
		converted.end = extensions.Time(result.EndTime)
		converted.extensionTestResult = result

		ret = append(ret, converted)
	}

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
	return result
}
