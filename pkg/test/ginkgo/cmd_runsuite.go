package ginkgo

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/onsi/ginkgo/v2"
	"github.com/openshift/origin/pkg/defaultmonitortests"
	"github.com/openshift/origin/pkg/disruption/backend/sampler"
	"github.com/openshift/origin/pkg/monitor"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/pkg/riskanalysis"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	// Dump pod displacements with at least 3 instances
	minChainLen = 3

	setupEvent       = "Setup"
	upgradeEvent     = "Upgrade"
	postUpgradeEvent = "PostUpgrade"
)

// GinkgoRunSuiteOptions is used to run a suite of tests by invoking each test
// as a call to a child worker (the run-tests command).
type GinkgoRunSuiteOptions struct {
	Parallelism int
	Count       int
	FailFast    bool
	Timeout     time.Duration
	JUnitDir    string

	// SyntheticEventTests allows the caller to translate events or outside
	// context into a failure.
	SyntheticEventTests JUnitsForEvents

	ClusterStabilityDuringTest string

	IncludeSuccessOutput bool

	CommandEnv []string

	DryRun        bool
	PrintCommands bool
	genericclioptions.IOStreams

	FromRepository string

	StartTime time.Time
}

func NewGinkgoRunSuiteOptions(streams genericclioptions.IOStreams) *GinkgoRunSuiteOptions {
	return &GinkgoRunSuiteOptions{
		IOStreams:                  streams,
		ClusterStabilityDuringTest: string(Stable),
	}
}

func (o *GinkgoRunSuiteOptions) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.DryRun, "dry-run", o.DryRun, "Print the tests to run without executing them.")
	flags.BoolVar(&o.PrintCommands, "print-commands", o.PrintCommands, "Print the sub-commands that would be executed instead.")
	flags.StringVar(&o.ClusterStabilityDuringTest, "cluster-stability", o.ClusterStabilityDuringTest, "cluster stability during test, usually dependent on the job: Stable or Disruptive")
	flags.StringVar(&o.JUnitDir, "junit-dir", o.JUnitDir, "The directory to write test reports to.")
	flags.IntVar(&o.Count, "count", o.Count, "Run each test a specified number of times. Defaults to 1 or the suite's preferred value. -1 will run forever.")
	flags.BoolVar(&o.FailFast, "fail-fast", o.FailFast, "If a test fails, exit immediately.")
	flags.DurationVar(&o.Timeout, "timeout", o.Timeout, "Set the maximum time a test can run before being aborted. This is read from the suite by default, but will be 10 minutes otherwise.")
	flags.BoolVar(&o.IncludeSuccessOutput, "include-success", o.IncludeSuccessOutput, "Print output from successful tests.")
	flags.IntVar(&o.Parallelism, "max-parallel-tests", o.Parallelism, "Maximum number of tests running in parallel. 0 defaults to test suite recommended value, which is different in each suite.")
}

func (o *GinkgoRunSuiteOptions) Validate() error {
	switch o.ClusterStabilityDuringTest {
	case string(Stable), string(Disruptive):
	default:
		return fmt.Errorf("unknown --cluster-stability, %q, expected Stable or Disruptive", o.ClusterStabilityDuringTest)
	}
	return nil
}

func (o *GinkgoRunSuiteOptions) AsEnv() []string {
	var args []string
	args = append(args, fmt.Sprintf("TEST_SUITE_START_TIME=%d", o.StartTime.Unix()))
	args = append(args, o.CommandEnv...)
	return args
}

func (o *GinkgoRunSuiteOptions) SetIOStreams(streams genericclioptions.IOStreams) {
	o.IOStreams = streams
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (o *GinkgoRunSuiteOptions) Run(suite *TestSuite, junitSuiteName string, monitorTestInfo monitortestframework.MonitorTestInitializationInfo, upgrade bool) error {
	ctx := context.Background()

	tests, err := testsForSuite()
	if err != nil {
		return fmt.Errorf("failed reading origin test suites: %w", err)
	}

	var fallbackSyntheticTestResult []*junitapi.JUnitTestCase
	// OPENSHIFT_SKIP_EXTERNAL_TESTS env variable allows to skip using external binary
	// in a similar fashion when --from-repository flag is specified when invoking tests
	// this means that images are very likely mirrored so for the time being we cannot
	// use external binary for tests
	// TODO (soltysh): when using external binary we should also consult that binary
	// for the list of tests it might require to run them
	if len(os.Getenv("OPENSHIFT_SKIP_EXTERNAL_TESTS")) == 0 &&
		strings.EqualFold(o.FromRepository, "quay.io/openshift/community-e2e-images") {
		buf := &bytes.Buffer{}
		fmt.Fprintf(buf, "Attempting to pull tests from external binary...\n")
		externalTests, err := externalTestsForSuite(ctx)
		if err == nil {
			filteredTests := []*testCase{}
			for _, test := range tests {
				// tests contains all the tests "registered" in openshif-tests binary,
				// this also includes vendored k8s tests, since this path assumes we're
				// using external binary to run these tests we need to remove them
				// from the final lists, which contains:
				// 1. origin tests, only
				// 2. k8s tests, coming from external binary
				if !strings.Contains(test.name, "[Suite:k8s]") {
					filteredTests = append(filteredTests, test)
				}
			}
			tests = append(filteredTests, externalTests...)
			fmt.Fprintf(buf, "Got %d tests from external binary\n", len(externalTests))
		} else {
			fmt.Fprintf(buf, "Falling back to built-in suite, failed reading external test suites: %v\n", err)
			// adding this test twice (one failure here, and success below) will
			// ensure it gets picked as flake further down in synthetic tests processing
			fallbackSyntheticTestResult = append(fallbackSyntheticTestResult, &junitapi.JUnitTestCase{
				Name:      "[sig-arch] External binary usage",
				SystemOut: buf.String(),
				FailureOutput: &junitapi.FailureOutput{
					Output: buf.String(),
				},
			})
		}
		fmt.Fprintf(o.Out, buf.String())
		fallbackSyntheticTestResult = append(fallbackSyntheticTestResult, &junitapi.JUnitTestCase{
			Name:      "[sig-arch] External binary usage",
			SystemOut: buf.String(),
		})
	} else {
		fmt.Fprintf(o.Out, "Using built-in tests only due to OPENSHIFT_SKIP_EXTERNAL_TESTS being set or --from-repository=%s not being the default\n", o.FromRepository)
	}

	// this ensures the tests are always run in random order to avoid
	// any intra-tests dependencies
	suiteConfig, _ := ginkgo.GinkgoConfiguration()
	r := rand.New(rand.NewSource(suiteConfig.RandomSeed))
	r.Shuffle(len(tests), func(i, j int) { tests[i], tests[j] = tests[j], tests[i] })

	tests = suite.Filter(tests)
	if len(tests) == 0 {
		return fmt.Errorf("suite %q does not contain any tests", suite.Name)
	}

	count := o.Count
	if count == 0 {
		count = suite.Count
	}

	start := time.Now()
	if o.StartTime.IsZero() {
		o.StartTime = start
	}

	timeout := o.Timeout
	if timeout == 0 {
		timeout = suite.TestTimeout
	}
	if timeout == 0 {
		timeout = 15 * time.Minute
	}

	testRunnerContext := newCommandContext(o.AsEnv(), timeout)

	if o.PrintCommands {
		newParallelTestQueue(testRunnerContext).OutputCommands(ctx, tests, o.Out)
		return nil
	}
	if o.DryRun {
		for _, test := range sortedTests(tests) {
			fmt.Fprintf(o.Out, "%q\n", test.name)
		}
		return nil
	}

	if len(o.JUnitDir) > 0 {
		if _, err := os.Stat(o.JUnitDir); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("could not access --junit-dir: %v", err)
			}
			if err := os.MkdirAll(o.JUnitDir, 0755); err != nil {
				return fmt.Errorf("could not create --junit-dir: %v", err)
			}
		}
	}

	parallelism := o.Parallelism
	if parallelism == 0 {
		parallelism = suite.Parallelism
	}
	if parallelism == 0 {
		parallelism = 10
	}

	restConfig, err := monitor.GetMonitorRESTConfig()
	if err != nil {
		return err
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal, 2)
	go func() {
		<-abortCh
		fmt.Fprintf(o.ErrOut, "Interrupted, terminating tests\n")
		sampler.TearDownInClusterMonitors(restConfig)
		cancelFn()
		sig := <-abortCh
		fmt.Fprintf(o.ErrOut, "Interrupted twice, exiting (%s)\n", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	monitorEventRecorder := monitor.NewRecorder()
	m := monitor.NewMonitor(
		monitorEventRecorder,
		restConfig,
		o.JUnitDir,
		defaultmonitortests.NewMonitorTestsFor(monitorTestInfo),
	)
	if err := m.Start(ctx); err != nil {
		return err
	}

	pc, err := SetupNewPodCollector(ctx)
	if err != nil {
		return err
	}

	pc.SetEvents([]string{setupEvent})
	pc.Run(ctx)

	// if we run a single test, always include success output
	includeSuccess := o.IncludeSuccessOutput
	if len(tests) == 1 && count == 1 {
		includeSuccess = true
	}
	testOutputLock := &sync.Mutex{}
	testOutputConfig := newTestOutputConfig(testOutputLock, o.Out, monitorEventRecorder, includeSuccess)

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
	if o.FailFast {
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
	if len(o.JUnitDir) > 0 {
		pc.ComputePodTransitions()
		data, err := pc.JsonDump()
		if err != nil {
			fmt.Fprintf(o.ErrOut, "Unable to dump pod placement data: %v\n", err)
		} else {
			if err := ioutil.WriteFile(filepath.Join(o.JUnitDir, "pod-placement-data.json"), data, 0644); err != nil {
				fmt.Fprintf(o.ErrOut, "Unable to write pod placement data: %v\n", err)
			}
		}
		chains := pc.PodDisplacements().Dump(minChainLen)
		if err := ioutil.WriteFile(filepath.Join(o.JUnitDir, "pod-transitions.txt"), []byte(chains), 0644); err != nil {
			fmt.Fprintf(o.ErrOut, "Unable to write pod placement data: %v\n", err)
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

		fmt.Fprintf(o.Out, "Retry count: %d\n", len(retries))

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
				fmt.Fprintf(o.Out, "Ignoring retry that returned a flake, original failure is authoritative for test: %s\n", retry.name)
				continue
			}
			tests = append(tests, retry)
		}
		if len(flaky) > 0 {
			failing = repeatFailures
			sort.Strings(flaky)
			fmt.Fprintf(o.Out, "Flaky tests:\n\n%s\n\n", strings.Join(flaky, "\n"))
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
			fmt.Fprintf(o.Out, "Skipped tests that failed a precondition:\n\n%s\n\n", strings.Join(skipped, "\n"))

		}
	}

	// Fetch data from in-cluster monitors if available
	if err = sampler.TearDownInClusterMonitors(restConfig); err != nil {
		fmt.Printf("Failed to write events from in-cluster monitors, err: %v\n", err)
	}

	// monitor the cluster while the tests are running and report any detected anomalies
	var syntheticTestResults []*junitapi.JUnitTestCase
	var syntheticFailure bool

	timeSuffix := fmt.Sprintf("_%s", start.UTC().Format("20060102-150405"))

	monitorTestResultState, err := m.Stop(ctx)
	if err != nil {
		fmt.Fprintf(o.ErrOut, "error: Failed to stop monitor test: %v\n", err)
		monitorTestResultState = monitor.Failed
	}
	if err := m.SerializeResults(ctx, junitSuiteName, timeSuffix); err != nil {
		fmt.Fprintf(o.ErrOut, "error: Failed to serialize run-data: %v\n", err)
	}

	// default is empty string as that is what entries prior to adding this will have
	wasMasterNodeUpdated := ""
	if events := monitorEventRecorder.Intervals(start, end); len(events) > 0 {
		buf := &bytes.Buffer{}
		if !upgrade {
			// the current mechanism for external binaries does not support upgrade
			// tests, so don't report information there at all
			syntheticTestResults = append(syntheticTestResults, fallbackSyntheticTestResult...)
		}

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
		if len(o.JUnitDir) > 0 {
			filename := fmt.Sprintf("openshift-tests-monitor_%s.txt", o.StartTime.UTC().Format("20060102-150405"))
			if err := ioutil.WriteFile(filepath.Join(o.JUnitDir, filename), buf.Bytes(), 0644); err != nil {
				fmt.Fprintf(o.ErrOut, "error: Failed to write monitor data: %v\n", err)
			}

			filename = fmt.Sprintf("events_used_for_junits_%s.json", o.StartTime.UTC().Format("20060102-150405"))
			if err := monitorserialization.EventsToFile(filepath.Join(o.JUnitDir, filename), events); err != nil {
				fmt.Fprintf(o.ErrOut, "error: Failed to junit event info: %v\n", err)
			}
		}

		wasMasterNodeUpdated = monitor.WasMasterNodeUpdated(events)
	}

	// report the outcome of the test
	if len(failing) > 0 {
		names := sets.NewString(testNames(failing)...).List()
		fmt.Fprintf(o.Out, "Failing tests:\n\n%s\n\n", strings.Join(names, "\n"))
	}

	if len(o.JUnitDir) > 0 {
		finalSuiteResults := generateJUnitTestSuiteResults(junitSuiteName, duration, tests, syntheticTestResults...)
		if err := writeJUnitReport(finalSuiteResults, "junit_e2e", timeSuffix, o.JUnitDir, o.ErrOut); err != nil {
			fmt.Fprintf(o.Out, "error: Unable to write e2e JUnit xml results: %v", err)
		}

		if err := riskanalysis.WriteJobRunTestFailureSummary(o.JUnitDir, timeSuffix, finalSuiteResults, wasMasterNodeUpdated); err != nil {
			fmt.Fprintf(o.Out, "error: Unable to write e2e job run failures summary: %v", err)
		}
	}

	if fail > 0 {
		if len(failing) > 0 || suite.MaximumAllowedFlakes == 0 {
			return fmt.Errorf("%d fail, %d pass, %d skip (%s)", fail, pass, skip, duration)
		}
		fmt.Fprintf(o.Out, "%d flakes detected, suite allows passing with only flakes\n\n", fail)
	}

	if syntheticFailure {
		return fmt.Errorf("failed because an invariant was violated, %d pass, %d skip (%s)\n", pass, skip, duration)
	}
	if monitorTestResultState != monitor.Succeeded {
		return fmt.Errorf("failed due to a MonitorTest failure")
	}

	fmt.Fprintf(o.Out, "%d pass, %d skip (%s)\n", pass, skip, duration)
	return ctx.Err()
}
