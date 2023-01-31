package ginkgo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/riskanalysis"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

const (
	// Dump pod displacements with at least 3 instances
	minChainLen = 3

	setupEvent       = "Setup"
	upgradeEvent     = "Upgrade"
	postUpgradeEvent = "PostUpgrade"
)

// Options is used to run a suite of tests by invoking each test
// as a call to a child worker (the run-tests command).
type Options struct {
	Parallelism int
	Count       int
	FailFast    bool
	Timeout     time.Duration
	JUnitDir    string
	TestFile    string
	OutFile     string

	// Regex allows a selection of a subset of tests
	Regex string
	// MatchFn if set is also used to filter the suite contents
	MatchFn func(name string) bool

	// SyntheticEventTests allows the caller to translate events or outside
	// context into a failure.
	SyntheticEventTests JUnitsForEvents

	MonitorEventsOptions *MonitorEventsOptions

	IncludeSuccessOutput bool

	CommandEnv []string

	DryRun        bool
	PrintCommands bool
	Out, ErrOut   io.Writer

	StartTime time.Time
}

func NewOptions(out io.Writer, errOut io.Writer) *Options {
	return &Options{
		MonitorEventsOptions: NewMonitorEventsOptions(out, errOut),
		Out:                  out,
		ErrOut:               errOut,
	}
}

func (opt *Options) AsEnv() []string {
	var args []string
	args = append(args, fmt.Sprintf("TEST_SUITE_START_TIME=%d", opt.StartTime.Unix()))
	args = append(args, opt.CommandEnv...)
	return args
}

func (opt *Options) SelectSuite(suites []*TestSuite, args []string) (*TestSuite, error) {
	var suite *TestSuite

	// If a test file was provided with no suite, use the "files" suite.
	if len(opt.TestFile) > 0 && len(args) == 0 {
		suite = &TestSuite{
			Name: "files",
		}
	}
	if suite == nil && len(args) == 0 {
		fmt.Fprintf(opt.ErrOut, SuitesString(suites, "Select a test suite to run against the server:\n\n"))
		return nil, fmt.Errorf("specify a test suite to run, for example: %s run %s", filepath.Base(os.Args[0]), suites[0].Name)
	}
	if suite == nil && len(args) > 0 {
		for _, s := range suites {
			if s.Name == args[0] {
				suite = s
				break
			}
		}
	}
	if suite == nil {
		fmt.Fprintf(opt.ErrOut, SuitesString(suites, "Select a test suite to run against the server:\n\n"))
		return nil, fmt.Errorf("suite %q does not exist", args[0])
	}
	// If a test file was provided, override the Matches function
	// to match the tests from both the suite and the file.
	if len(opt.TestFile) > 0 {
		var in []byte
		var err error
		if opt.TestFile == "-" {
			in, err = ioutil.ReadAll(os.Stdin)
			if err != nil {
				return nil, err
			}
		} else {
			in, err = ioutil.ReadFile(opt.TestFile)
		}
		if err != nil {
			return nil, err
		}
		err = matchTestsFromFile(suite, in)
		if err != nil {
			return nil, fmt.Errorf("could not read test suite from input: %v", err)
		}
	}
	return suite, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (opt *Options) Run(suite *TestSuite, junitSuiteName string) error {
	ctx := context.Background()

	if len(opt.Regex) > 0 {
		if err := filterWithRegex(suite, opt.Regex); err != nil {
			return err
		}
	}
	if opt.MatchFn != nil {
		original := suite.Matches
		suite.Matches = func(name string) bool {
			return original(name) && opt.MatchFn(name)
		}
	}

	syntheticEventTests := JUnitsForAllEvents{
		opt.SyntheticEventTests,
		suite.SyntheticEventTests,
	}

	tests, err := testsForSuite()
	if err != nil {
		return err
	}

	discoveryClient, err := getDiscoveryClient()
	if err != nil {
		if opt.DryRun {
			fmt.Fprintf(opt.ErrOut, "Unable to get discovery client, skipping apigroup check in the dry-run mode: %v\n", err)
		} else {
			return err
		}
	} else {
		if _, err := discoveryClient.ServerVersion(); err != nil {
			if opt.DryRun {
				fmt.Fprintf(opt.ErrOut, "Unable to get server version through discovery client, skipping apigroup check in the dry-run mode: %v\n", err)
			} else {
				return err
			}
		} else {
			apiGroupFilter, err := newApiGroupFilter(discoveryClient)
			if err != nil {
				return fmt.Errorf("unable to build api group filter: %v", err)
			}

			// Skip tests with [apigroup:GROUP] labels for apigroups which are not
			// served by a cluster. E.g. MicroShift is not serving most of the openshift.io
			// apigroups. Other installations might be serving only a subset of the api groups.
			apiGroupFilter.markSkippedWhenAPIGroupNotServed(tests)
		}
	}

	tests = suite.Filter(tests)
	if len(tests) == 0 {
		return fmt.Errorf("suite %q does not contain any tests", suite.Name)
	}

	count := opt.Count
	if count == 0 {
		count = suite.Count
	}

	start := time.Now()
	if opt.StartTime.IsZero() {
		opt.StartTime = start
	}

	timeout := opt.Timeout
	if timeout == 0 {
		timeout = suite.TestTimeout
	}
	if timeout == 0 {
		timeout = 15 * time.Minute
	}

	testRunnerContext := newCommandContext(opt.AsEnv(), timeout)

	if opt.PrintCommands {
		newParallelTestQueue(testRunnerContext).OutputCommands(ctx, tests, opt.Out)
		return nil
	}
	if opt.DryRun {
		for _, test := range sortedTests(tests) {
			fmt.Fprintf(opt.Out, "%q\n", test.name)
		}
		return nil
	}

	if len(opt.JUnitDir) > 0 {
		if _, err := os.Stat(opt.JUnitDir); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("could not access --junit-dir: %v", err)
			}
			if err := os.MkdirAll(opt.JUnitDir, 0755); err != nil {
				return fmt.Errorf("could not create --junit-dir: %v", err)
			}
		}
	}

	parallelism := opt.Parallelism
	if parallelism == 0 {
		parallelism = suite.Parallelism
	}
	if parallelism == 0 {
		parallelism = 10
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal, 2)
	go func() {
		<-abortCh
		fmt.Fprintf(opt.ErrOut, "Interrupted, terminating tests\n")
		cancelFn()
		sig := <-abortCh
		fmt.Fprintf(opt.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	restConfig, err := monitor.GetMonitorRESTConfig()
	if err != nil {
		return err
	}
	monitorEventRecorder, err := opt.MonitorEventsOptions.Start(ctx, restConfig)
	if err != nil {
		return err
	}

	pc, err := SetupNewPodCollector(ctx)
	if err != nil {
		return err
	}

	pc.SetEvents([]string{setupEvent})
	pc.Run(ctx)

	// if we run a single test, always include success output
	includeSuccess := opt.IncludeSuccessOutput
	if len(tests) == 1 && count == 1 {
		includeSuccess = true
	}
	testOutputLock := &sync.Mutex{}
	testOutputConfig := newTestOutputConfig(testOutputLock, opt.Out, monitorEventRecorder, includeSuccess)

	early, notEarly := splitTests(tests, func(t *testCase) bool {
		return strings.Contains(t.name, "[Early]")
	})

	late, primaryTests := splitTests(notEarly, func(t *testCase) bool {
		return strings.Contains(t.name, "[Late]")
	})

	kubeTests, openshiftTests := splitTests(primaryTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[Suite:k8s]")
	})

	storageTests, kubeTests := splitTests(kubeTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-storage]")
	})

	mustGatherTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-cli] oc adm must-gather")
	})

	// If user specifies a count, duplicate the kube and openshift tests that many times.
	expectedTestCount := len(early) + len(late)
	if count != -1 {
		originalKube := kubeTests
		originalOpenshift := openshiftTests
		originalStorage := storageTests
		originalMustGather := mustGatherTests

		for i := 1; i < count; i++ {
			kubeTests = append(kubeTests, copyTests(originalKube)...)
			openshiftTests = append(openshiftTests, copyTests(originalOpenshift)...)
			storageTests = append(storageTests, copyTests(originalStorage)...)
			mustGatherTests = append(mustGatherTests, copyTests(originalMustGather)...)
		}
	}
	expectedTestCount += len(openshiftTests) + len(kubeTests) + len(storageTests) + len(mustGatherTests)

	abortFn := neverAbort
	testCtx := ctx
	if opt.FailFast {
		abortFn, testCtx = abortOnFailure(ctx)
	}

	tests = nil

	// run our Early tests
	q := newParallelTestQueue(testRunnerContext)
	q.Execute(testCtx, early, parallelism, testOutputConfig, abortFn)
	tests = append(tests, early...)

	// TODO: will move to the monitor
	pc.SetEvents([]string{upgradeEvent})

	// Run kube, storage, openshift, and must-gather tests. If user specified a count of -1,
	// we loop indefinitely.
	for i := 0; (i < 1 || count == -1) && testCtx.Err() == nil; i++ {
		kubeTestsCopy := copyTests(kubeTests)
		q.Execute(testCtx, kubeTestsCopy, parallelism, testOutputConfig, abortFn)
		tests = append(tests, kubeTestsCopy...)

		// I thought about randomizing the order of the kube, storage, and openshift tests, but storage dominates our e2e runs, so it doesn't help much.
		storageTestsCopy := copyTests(storageTests)
		q.Execute(testCtx, storageTestsCopy, max(1, parallelism/2), testOutputConfig, abortFn) // storage tests only run at half the parallelism, so we can avoid cloud provider quota problems.
		tests = append(tests, storageTestsCopy...)

		openshiftTestsCopy := copyTests(openshiftTests)
		q.Execute(testCtx, openshiftTestsCopy, parallelism, testOutputConfig, abortFn)
		tests = append(tests, openshiftTestsCopy...)

		// run the must-gather tests after parallel tests to reduce resource contention
		mustGatherTestsCopy := copyTests(mustGatherTests)
		q.Execute(testCtx, mustGatherTestsCopy, parallelism, testOutputConfig, abortFn)
		tests = append(tests, mustGatherTestsCopy...)
	}

	// TODO: will move to the monitor
	pc.SetEvents([]string{postUpgradeEvent})

	// run Late test suits after everything else
	q.Execute(testCtx, late, parallelism, testOutputConfig, abortFn)
	tests = append(tests, late...)

	// TODO: will move to the monitor
	if len(opt.JUnitDir) > 0 {
		pc.ComputePodTransitions()
		data, err := pc.JsonDump()
		if err != nil {
			fmt.Fprintf(opt.ErrOut, "Unable to dump pod placement data: %v\n", err)
		} else {
			if err := ioutil.WriteFile(filepath.Join(opt.JUnitDir, "pod-placement-data.json"), data, 0644); err != nil {
				fmt.Fprintf(opt.ErrOut, "Unable to write pod placement data: %v\n", err)
			}
		}
		chains := pc.PodDisplacements().Dump(minChainLen)
		if err := ioutil.WriteFile(filepath.Join(opt.JUnitDir, "pod-transitions.txt"), []byte(chains), 0644); err != nil {
			fmt.Fprintf(opt.ErrOut, "Unable to write pod placement data: %v\n", err)
		}
	}

	// calculate the effective test set we ran, excluding any incompletes
	tests, _ = splitTests(tests, func(t *testCase) bool { return t.success || t.flake || t.failed || t.skipped })

	end := time.Now()
	duration := end.Sub(start).Round(time.Second / 10)
	if duration > time.Minute {
		duration = duration.Round(time.Second)
	}

	pass, fail, skip, failing := summarizeTests(tests)

	// attempt to retry failures to do flake detection
	if fail > 0 && fail <= suite.MaximumAllowedFlakes {
		var retries []*testCase

		// Make a copy of the all failing tests (subject to the max allowed flakes) so we can have
		// a list of tests to retry.
		for _, test := range failing {
			retry := test.Retry()
			retries = append(retries, retry)
			if len(retries) > suite.MaximumAllowedFlakes {
				break
			}
		}

		fmt.Fprintf(opt.Out, "Retry count: %d\n", len(retries))

		// Run the tests in the retries list.
		q := newParallelTestQueue(testRunnerContext)
		q.Execute(testCtx, retries, parallelism, testOutputConfig, abortFn)

		var flaky, skipped []string
		var repeatFailures []*testCase
		for _, test := range retries {
			if test.success {
				flaky = append(flaky, test.name)
			} else if test.skipped {
				skipped = append(skipped, test.name)
			} else {
				repeatFailures = append(repeatFailures, test)
			}
		}

		// Add the list of retries into the list of all tests.
		for _, retry := range retries {
			if retry.flake {
				// Retry tests that flaked are omitted so that the original test is counted as a failure.
				fmt.Fprintf(opt.Out, "Ignoring retry that returned a flake, original failure is authoritative for test: %s\n", retry.name)
				continue
			}
			tests = append(tests, retry)
		}
		if len(flaky) > 0 {
			failing = repeatFailures
			sort.Strings(flaky)
			fmt.Fprintf(opt.Out, "Flaky tests:\n\n%s\n\n", strings.Join(flaky, "\n"))
		}
		if len(skipped) > 0 {
			// If a retry test got skipped, it means we very likely failed a precondition in the first failure, so
			// we need to remove the failure case.
			var withoutPreconditionFailures []*testCase
		testLoop:
			for _, t := range tests {
				for _, st := range skipped {
					if t.name == st && t.failed {
						continue testLoop
					}
					withoutPreconditionFailures = append(withoutPreconditionFailures, t)
				}
			}
			tests = withoutPreconditionFailures
			failing = repeatFailures
			sort.Strings(skipped)
			fmt.Fprintf(opt.Out, "Skipped tests that failed a precondition:\n\n%s\n\n", strings.Join(skipped, "\n"))

		}
	}

	// monitor the cluster while the tests are running and report any detected anomalies
	var syntheticTestResults []*junitapi.JUnitTestCase
	var syntheticFailure bool

	timeSuffix := fmt.Sprintf("_%s", opt.MonitorEventsOptions.GetStartTime().
		UTC().Format("20060102-150405"))

	if err := opt.MonitorEventsOptions.End(ctx, restConfig, opt.JUnitDir); err != nil {
		return err
	}
	if len(opt.JUnitDir) > 0 {
		if err := opt.MonitorEventsOptions.WriteRunDataToArtifactsDir(opt.JUnitDir, timeSuffix); err != nil {
			fmt.Fprintf(opt.ErrOut, "error: Failed to write run-data: %v\n", err)
		}
	}

	if events := opt.MonitorEventsOptions.GetEvents(); len(events) > 0 {
		var buf *bytes.Buffer
		syntheticTestResults, buf, _ = createSyntheticTestsFromMonitor(events, duration)
		currResState := opt.MonitorEventsOptions.GetRecordedResources()
		testCases := syntheticEventTests.JUnitsForEvents(events, duration, restConfig, suite.Name, &currResState)
		syntheticTestResults = append(syntheticTestResults, testCases...)

		if len(syntheticTestResults) > 0 {
			// mark any failures by name
			failingSyntheticTestNames, flakySyntheticTestNames := sets.NewString(), sets.NewString()
			for _, test := range syntheticTestResults {
				if test.FailureOutput != nil {
					failingSyntheticTestNames.Insert(test.Name)
				}
			}
			// if a test has both a pass and a failure, flag it
			// as a flake
			for _, test := range syntheticTestResults {
				if test.FailureOutput == nil {
					if failingSyntheticTestNames.Has(test.Name) {
						flakySyntheticTestNames.Insert(test.Name)
					}
				}
			}
			failingSyntheticTestNames = failingSyntheticTestNames.Difference(flakySyntheticTestNames)
			if failingSyntheticTestNames.Len() > 0 {
				fmt.Fprintf(buf, "Failing invariants:\n\n%s\n\n", strings.Join(failingSyntheticTestNames.List(), "\n"))
				syntheticFailure = true
			}
			if flakySyntheticTestNames.Len() > 0 {
				fmt.Fprintf(buf, "Flaky invariants:\n\n%s\n\n", strings.Join(flakySyntheticTestNames.List(), "\n"))
			}
		}

		// we only write the buffer if we have an artifact location
		if len(opt.JUnitDir) > 0 {
			filename := fmt.Sprintf("openshift-tests-monitor_%s.txt", opt.StartTime.UTC().Format("20060102-150405"))
			if err := ioutil.WriteFile(filepath.Join(opt.JUnitDir, filename), buf.Bytes(), 0644); err != nil {
				fmt.Fprintf(opt.ErrOut, "error: Failed to write monitor data: %v\n", err)
			}
		}
	}

	// report the outcome of the test
	if len(failing) > 0 {
		names := sets.NewString(testNames(failing)...).List()
		fmt.Fprintf(opt.Out, "Failing tests:\n\n%s\n\n", strings.Join(names, "\n"))
	}

	if len(opt.JUnitDir) > 0 {
		finalSuiteResults := generateJUnitTestSuiteResults(junitSuiteName, duration, tests, syntheticTestResults...)
		if err := writeJUnitReport(finalSuiteResults, "junit_e2e", timeSuffix, opt.JUnitDir, opt.ErrOut); err != nil {
			fmt.Fprintf(opt.Out, "error: Unable to write e2e JUnit xml results: %v", err)
		}

		if err := riskanalysis.WriteJobRunTestFailureSummary(opt.JUnitDir, timeSuffix, finalSuiteResults); err != nil {
			fmt.Fprintf(opt.Out, "error: Unable to write e2e job run failures summary: %v", err)
		}
	}

	if fail > 0 {
		if len(failing) > 0 || suite.MaximumAllowedFlakes == 0 {
			return fmt.Errorf("%d fail, %d pass, %d skip (%s)", fail, pass, skip, duration)
		}
		fmt.Fprintf(opt.Out, "%d flakes detected, suite allows passing with only flakes\n\n", fail)
	}

	if syntheticFailure {
		return fmt.Errorf("failed because an invariant was violated, %d pass, %d skip (%s)\n", pass, skip, duration)
	}

	fmt.Fprintf(opt.Out, "%d pass, %d skip (%s)\n", pass, skip, duration)
	return ctx.Err()
}
