package ginkgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	"github.com/onsi/ginkgo/v2"
	configv1 "github.com/openshift/api/config/v1"
	clientconfigv1 "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/mod/semver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/clioptions/clusterinfo"
	"github.com/openshift/origin/pkg/clioptions/kubeconfig"
	"github.com/openshift/origin/pkg/defaultmonitortests"
	"github.com/openshift/origin/pkg/monitor"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/riskanalysis"
	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
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

	// ShardCount is the total number of partitions the test suite is divided into.
	// Each executor runs one of these partitions.
	ShardCount int

	// ShardStrategy is which strategy we'll use for dividing tests.
	ShardStrategy string

	// ShardID is the 1-based index of the shard this instance is responsible for running.
	ShardID int

	// SyntheticEventTests allows the caller to translate events or outside
	// context into a failure.
	SyntheticEventTests JUnitsForEvents

	ClusterStabilityDuringTest string

	IncludeSuccessOutput bool

	CommandEnv []string

	DryRun        bool
	PrintCommands bool
	genericclioptions.IOStreams

	StartTime time.Time

	ExactMonitorTests   []string
	DisableMonitorTests []string
}

func NewGinkgoRunSuiteOptions(streams genericclioptions.IOStreams) *GinkgoRunSuiteOptions {
	return &GinkgoRunSuiteOptions{
		IOStreams:     streams,
		ShardStrategy: "hash",
	}
}

func (o *GinkgoRunSuiteOptions) BindFlags(flags *pflag.FlagSet) {

	monitorNames := defaultmonitortests.ListAllMonitorTests()

	flags.BoolVar(&o.DryRun, "dry-run", o.DryRun, "Print the tests to run without executing them.")
	flags.BoolVar(&o.PrintCommands, "print-commands", o.PrintCommands, "Print the sub-commands that would be executed instead.")
	flags.StringVar(&o.ClusterStabilityDuringTest, "cluster-stability", o.ClusterStabilityDuringTest, "cluster stability during test, usually dependent on the job: Stable or Disruptive. Empty default will be treated as Stable.")
	flags.StringVar(&o.JUnitDir, "junit-dir", o.JUnitDir, "The directory to write test reports to.")
	flags.IntVar(&o.Count, "count", o.Count, "Run each test a specified number of times. Defaults to 1 or the suite's preferred value. -1 will run forever.")
	flags.BoolVar(&o.FailFast, "fail-fast", o.FailFast, "If a test fails, exit immediately.")
	flags.DurationVar(&o.Timeout, "timeout", o.Timeout, "Set the maximum time a test can run before being aborted. This is read from the suite by default, but will be 10 minutes otherwise.")
	flags.BoolVar(&o.IncludeSuccessOutput, "include-success", o.IncludeSuccessOutput, "Print output from successful tests.")
	flags.IntVar(&o.Parallelism, "max-parallel-tests", o.Parallelism, "Maximum number of tests running in parallel. 0 defaults to test suite recommended value, which is different in each suite.")
	flags.StringSliceVar(&o.ExactMonitorTests, "monitor", o.ExactMonitorTests,
		fmt.Sprintf("list of exactly which monitors to enable. All others will be disabled.  Current monitors are: [%s]", strings.Join(monitorNames, ", ")))
	flags.StringSliceVar(&o.DisableMonitorTests, "disable-monitor", o.DisableMonitorTests, "list of monitors to disable.  Defaults for others will be honored.")

	flags.IntVar(&o.ShardID, "shard-id", o.ShardID, "When tests are sharded across instances, which instance we are")
	flags.IntVar(&o.ShardCount, "shard-count", o.ShardCount, "Number of shards used to run tests across multiple instances")
	flags.StringVar(&o.ShardStrategy, "shard-strategy", o.ShardStrategy, "Which strategy to use for sharding (hash)")
}

func (o *GinkgoRunSuiteOptions) Validate() error {
	switch o.ClusterStabilityDuringTest {
	case "", string(Stable), string(Disruptive):
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

func (o *GinkgoRunSuiteOptions) Run(suite *TestSuite, junitSuiteName string, monitorTestInfo monitortestframework.MonitorTestInitializationInfo,
	upgrade bool) error {
	ctx := context.Background()

	tests, err := testsForSuite()
	if err != nil {
		return fmt.Errorf("failed reading origin test suites: %w", err)
	}

	var sharder Sharder
	switch o.ShardStrategy {
	default:
		sharder = &HashSharder{}
	}

	logrus.WithField("suite", suite.Name).Infof("Found %d internal tests in openshift-tests binary", len(tests))

	var fallbackSyntheticTestResult []*junitapi.JUnitTestCase
	var externalTestCases []*testCase
	if len(os.Getenv("OPENSHIFT_SKIP_EXTERNAL_TESTS")) == 0 {
		// Extract all test binaries
		extractionContext, extractionContextCancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer extractionContextCancel()
		cleanUpFn, externalBinaries, err := extensions.ExtractAllTestBinaries(extractionContext, 10)
		if err != nil {
			return err
		}
		defer cleanUpFn()

		defaultBinaryParallelism := 10

		// Learn about the extension binaries available
		// TODO(stbenjam): we'll eventually use this information to get suite information -- but not yet in this iteration
		infoContext, infoContextCancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer infoContextCancel()
		extensionsInfo, err := externalBinaries.Info(infoContext, defaultBinaryParallelism)
		if err != nil {
			return err
		}
		logrus.Infof("Discovered %d extensions", len(extensionsInfo))
		for _, e := range extensionsInfo {
			id := fmt.Sprintf("%s:%s:%s", e.Component.Product, e.Component.Kind, e.Component.Name)
			logrus.Infof("Extension %s found in %s:%s using API version %s", id, e.Source.SourceImage, e.Source.SourceBinary, e.APIVersion)
		}

		// List tests from all available binaries and convert them to origin's testCase format
		listContext, listContextCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer listContextCancel()

		envFlags, err := determineEnvironmentFlags(ctx, upgrade, o.DryRun)
		if err != nil {
			return fmt.Errorf("could not determine environment flags: %w", err)
		}
		logrus.WithFields(envFlags.LogFields()).Infof("Determined all potential environment flags")

		externalTestSpecs, err := externalBinaries.ListTests(listContext, defaultBinaryParallelism, envFlags)
		if err != nil {
			return err
		}
		externalTestCases = externalBinaryTestsToOriginTestCases(externalTestSpecs)

		var filteredTests []*testCase
		for _, test := range tests {
			// tests contains all the tests "registered" in openshift-tests binary,
			// this also includes vendored k8s tests, since this path assumes we're
			// using external binary to run these tests we need to remove them
			// from the final lists, which contains:
			// 1. origin tests, only
			// 2. k8s tests, coming from external binary
			if !strings.Contains(test.name, "[Suite:k8s]") {
				filteredTests = append(filteredTests, test)
			}
		}
		logrus.Infof("Discovered %d internal tests, %d external tests - %d total unique tests",
			len(tests), len(externalTestCases), len(filteredTests)+len(externalTestCases))
		tests = append(filteredTests, externalTestCases...)
	} else {
		logrus.Infof("Using built-in tests only due to OPENSHIFT_SKIP_EXTERNAL_TESTS being set")
	}

	// Temporarily check for the presence of the [Skipped:xyz] annotation in the test names, once this synthetic test
	// begins to pass we can remove the annotation logic
	var annotatedSkipped []string
	for _, t := range externalTestCases {
		if strings.Contains(t.name, "[Skipped") {
			annotatedSkipped = append(annotatedSkipped, t.name)
		}
	}
	var skippedAnnotationSyntheticTestResults []*junitapi.JUnitTestCase
	skippedAnnotationSyntheticTestResult := junitapi.JUnitTestCase{
		Name: "[sig-trt] Skipped annotations present",
	}
	if len(annotatedSkipped) > 0 {
		skippedAnnotationSyntheticTestResult.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("Skipped Annotations present in tests: %s", strings.Join(annotatedSkipped, ", ")),
		}
	}
	skippedAnnotationSyntheticTestResults = append(skippedAnnotationSyntheticTestResults, &skippedAnnotationSyntheticTestResult)
	// If this fails, this additional run will make it flake
	skippedAnnotationSyntheticTestResults = append(skippedAnnotationSyntheticTestResults, &junitapi.JUnitTestCase{Name: skippedAnnotationSyntheticTestResult.Name})

	// this ensures the tests are always run in random order to avoid
	// any intra-tests dependencies
	suiteConfig, _ := ginkgo.GinkgoConfiguration()
	r := rand.New(rand.NewSource(suiteConfig.RandomSeed))
	r.Shuffle(len(tests), func(i, j int) { tests[i], tests[j] = tests[j], tests[i] })

	tests = suite.Filter(tests)
	if len(tests) == 0 {
		return fmt.Errorf("suite %q does not contain any tests", suite.Name)
	}

	logrus.Infof("Found %d filtered tests", len(tests))

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

	restConfig, err := clusterinfo.GetMonitorRESTConfig()
	if err != nil {
		return err
	}

	// skip tests due to newer k8s
	tests, err = o.filterOutRebaseTests(restConfig, tests)
	if err != nil {
		return err
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

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	abortCh := make(chan os.Signal, 2)
	go func() {
		<-abortCh
		logrus.Error("Interrupted, terminating tests")
		cancelFn()
		sig := <-abortCh
		logrus.Errorf("Interrupted twice, exiting (%s)", sig)
		switch sig {
		case syscall.SIGINT:
			os.Exit(130)
		default:
			os.Exit(0)
		}
	}()
	signal.Notify(abortCh, syscall.SIGINT, syscall.SIGTERM)

	logrus.Infof("Waiting for all cluster operators to become stable")
	stableClusterTestResults, err := clusterinfo.WaitForStableCluster(ctx, restConfig)
	if err != nil {
		logrus.Errorf("Error waiting for stable cluster: %v", err)
	}

	monitorTests, err := defaultmonitortests.NewMonitorTestsFor(monitorTestInfo)
	if err != nil {
		logrus.Errorf("Error getting monitor tests: %v", err)
	}

	monitorEventRecorder := monitor.NewRecorder()
	m := monitor.NewMonitor(
		monitorEventRecorder,
		restConfig,
		o.JUnitDir,
		monitorTests,
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
	logrus.Infof("Found %d early tests", len(early))

	late, primaryTests := splitTests(notEarly, func(t *testCase) bool {
		return strings.Contains(t.name, "[Late]")
	})
	logrus.Infof("Found %d late tests", len(late))

	// Sharding always runs early and late tests in every invocation. I think this
	// makes sense, because these tests may collect invariant data we want to know about
	// every run.
	primaryTests, err = sharder.Shard(primaryTests, o.ShardCount, o.ShardID)
	if err != nil {
		return err
	}

	kubeTests, openshiftTests := splitTests(primaryTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[Suite:k8s]")
	})

	storageTests, kubeTests := splitTests(kubeTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-storage]")
	})

	mustGatherTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-cli] oc adm must-gather")
	})

	logrus.Infof("Found %d openshift tests", len(openshiftTests))
	logrus.Infof("Found %d kube tests", len(kubeTests))
	logrus.Infof("Found %d storage tests", len(storageTests))
	logrus.Infof("Found %d must-gather tests", len(mustGatherTests))

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
			logrus.Errorf("Unable to dump pod placement data: %v", err)
		} else {
			if err := ioutil.WriteFile(filepath.Join(o.JUnitDir, "pod-placement-data.json"), data, 0644); err != nil {
				logrus.Errorf("Unable to write pod placement data: %v", err)
			}
		}
		chains := pc.PodDisplacements().Dump(minChainLen)
		if err := ioutil.WriteFile(filepath.Join(o.JUnitDir, "pod-transitions.txt"), []byte(chains), 0644); err != nil {
			logrus.Errorf("Unable to write pod placement data: %v", err)
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

		logrus.Warningf("Retry count: %d", len(retries))

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

	// monitor the cluster while the tests are running and report any detected anomalies
	var syntheticTestResults []*junitapi.JUnitTestCase
	var syntheticFailure bool
	syntheticTestResults = append(syntheticTestResults, stableClusterTestResults...)
	syntheticTestResults = append(syntheticTestResults, skippedAnnotationSyntheticTestResults...)

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

		wasMasterNodeUpdated = clusterinfo.WasMasterNodeUpdated(events)
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

		if err := writeExtensionTestResults(tests, o.JUnitDir, "extension_test_result_e2e", timeSuffix, o.ErrOut); err != nil {
			fmt.Fprintf(o.Out, "error: Unable to write e2e Extension Test Result JSON results: %v", err)
		}

		if err := riskanalysis.WriteJobRunTestFailureSummary(o.JUnitDir, timeSuffix, finalSuiteResults, wasMasterNodeUpdated, ""); err != nil {
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

func writeExtensionTestResults(tests []*testCase, dir, filePrefix, fileSuffix string, out io.Writer) error {
	// Ensure the directory exists
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		fmt.Fprintf(out, "Failed to create directory %s: %v\n", dir, err)
		return err
	}

	// Collect results into a slice
	var results extensions.ExtensionTestResults
	for _, test := range tests {
		if test.extensionTestResult != nil {
			results = append(results, test.extensionTestResult)
		}
	}

	// Marshal results to JSON
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Fprintf(out, "Failed to marshal test results to JSON: %v\n", err)
		return err
	}

	// Write JSON data to file
	filePath := filepath.Join(dir, fmt.Sprintf("%s_%s.json", filePrefix, fileSuffix))
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Fprintf(out, "Failed to create file %s: %v\n", filePath, err)
		return err
	}
	defer file.Close()

	fmt.Fprintf(out, "Writing extension test results JSON to %s\n", filePath)
	_, err = file.Write(data)
	if err != nil {
		fmt.Fprintf(out, "Failed to write to file %s: %v\n", filePath, err)
		return err
	}

	return nil
}

func (o *GinkgoRunSuiteOptions) filterOutRebaseTests(restConfig *rest.Config, tests []*testCase) ([]*testCase, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, err
	}
	// TODO: this version along with below exclusions lists needs to be updated
	// for the rebase in-progress.
	if !strings.HasPrefix(serverVersion.Minor, "31") {
		return tests, nil
	}

	// Below list should only be filled in when we're trying to land k8s rebase.
	// Don't pile them up!
	exclusions := []string{
		// affected by the available controller split https://github.com/kubernetes/kubernetes/pull/126149
		`[sig-api-machinery] health handlers should contain necessary checks`,
	}

	matches := make([]*testCase, 0, len(tests))
outerLoop:
	for _, test := range tests {
		for _, excl := range exclusions {
			if strings.Contains(test.name, excl) {
				fmt.Fprintf(o.Out, "Skipping %q due to rebase in-progress\n", test.name)
				continue outerLoop
			}
		}
		matches = append(matches, test)
	}
	return matches, nil
}

func determineEnvironmentFlags(ctx context.Context, upgrade bool, dryRun bool) (extensions.EnvironmentFlags, error) {
	restConfig, err := e2e.LoadConfig(true)
	if err != nil {
		logrus.WithError(err).Error("error calling e2e.LoadConfig")
		return nil, err
	}
	clusterState, err := clusterdiscovery.DiscoverClusterState(restConfig)
	if err != nil {
		logrus.WithError(err).Warn("error Discovering Cluster State, flags requiring it will not be present")
	}
	provider := os.Getenv("TEST_PROVIDER")
	if clusterState == nil { // If we know we cannot discover the clusterState, the provider must be set to "none" in order for the config to be loaded without error
		provider = "none"
	}
	config, err := clusterdiscovery.DecodeProvider(provider, dryRun, true, clusterState)
	if err != nil {
		logrus.WithError(err).Error("error determining information about the cluster")
		return nil, err
	}

	envFlagBuilder := &extensions.EnvironmentFlagsBuilder{}
	envFlagBuilder.
		AddPlatform(config.ProviderName).
		AddNetwork(config.NetworkPlugin).
		AddNetworkStack(config.IPFamily).
		AddExternalConnectivity(determineExternalConnectivity(config))

	clientConfig, err := clientconfigv1.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	discoveryClient, err := kubeconfig.NewDiscoveryGetter(restConfig).GetDiscoveryClient()
	if err != nil {
		return nil, err
	}
	apiGroups, err := determineEnabledAPIGroups(discoveryClient)
	if err != nil {
		return nil, errors.WithMessage(err, "couldn't determine api groups")
	}
	envFlagBuilder.AddAPIGroups(apiGroups.UnsortedList()...)

	if apiGroups.Has("config.openshift.io") {
		featureGates, err := determineEnabledFeatureGates(ctx, clientConfig)
		if err != nil {
			return nil, errors.WithMessage(err, "couldn't determine feature gates")
		}
		envFlagBuilder.AddFeatureGates(featureGates...)
	}

	//Additional flags can only be determined if we are able to obtain the clusterState
	if clusterState != nil {
		upgradeType := "None"
		if upgrade {
			upgradeType = determineUpgradeType(clusterState.Version.Status)
		}
		envFlagBuilder.AddUpgrade(upgradeType)

		arch := "Unknown"
		if len(clusterState.Masters.Items) > 0 {
			//TODO(sgoeddel): eventually, we may need to check every node and pass "multi" as the value if any of them differ from the masters
			arch = clusterState.Masters.Items[0].Status.NodeInfo.Architecture
		}
		envFlagBuilder.AddArchitecture(arch)

		for _, optionalCapability := range clusterState.OptionalCapabilities {
			envFlagBuilder.AddOptionalCapability(string(optionalCapability))
		}

		envFlagBuilder.
			AddTopology(clusterState.ControlPlaneTopology).
			AddVersion(clusterState.Version.Status.Desired.Version)
	}

	return envFlagBuilder.Build(), nil
}

func determineUpgradeType(versionStatus configv1.ClusterVersionStatus) string {
	history := versionStatus.History
	version := versionStatus.Desired.Version
	if len(history) > 1 { // If there aren't at least 2 versions in the history we don't have any way to determine the upgrade type
		mostRecent := history[1] // history[0] will be the desired version, so check one further back
		current := fmt.Sprintf("v%s", version)
		last := fmt.Sprintf("v%s", mostRecent.Version)
		if semver.Compare(semver.Major(current), semver.Major(last)) > 0 {
			return "Major"
		}
		if semver.Compare(semver.MajorMinor(current), semver.MajorMinor(last)) > 0 {
			return "Minor"
		}

		return "Micro"
	}

	return "Unknown"
}

func determineExternalConnectivity(clusterConfig *clusterdiscovery.ClusterConfiguration) string {
	if clusterConfig.Disconnected {
		return "Disconnected"
	}
	if clusterConfig.IsProxied {
		return "Proxied"
	}
	return "Direct"
}

func determineEnabledAPIGroups(discoveryClient discovery.AggregatedDiscoveryInterface) (sets.Set[string], error) {
	groups, err := discoveryClient.ServerGroups()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve served resources: %v", err)
	}
	apiGroups := sets.New[string]()
	for _, apiGroup := range groups.Groups {
		// ignore the empty group
		if apiGroup.Name == "" {
			continue
		}
		apiGroups.Insert(apiGroup.Name)
	}

	return apiGroups, nil
}

func determineEnabledFeatureGates(ctx context.Context, configClient clientconfigv1.Interface) ([]string, error) {
	featureGate, err := configClient.ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	desiredVersion := clusterVersion.Status.Desired.Version
	if len(desiredVersion) == 0 && len(clusterVersion.Status.History) > 0 {
		desiredVersion = clusterVersion.Status.History[0].Version
	}

	ret := sets.NewString()
	found := false
	for _, featureGateValues := range featureGate.Status.FeatureGates {
		if featureGateValues.Version != desiredVersion {
			continue
		}
		found = true
		for _, enabled := range featureGateValues.Enabled {
			ret.Insert(string(enabled.Name))
		}
		break
	}
	if !found {
		logrus.Warning("no feature gates found")
		return nil, nil
	}

	return ret.List(), nil
}
