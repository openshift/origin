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
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/mod/semver"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/clioptions/clusterinfo"
	"github.com/openshift/origin/pkg/defaultmonitortests"
	"github.com/openshift/origin/pkg/monitor"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/riskanalysis"
	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/pkg/test/filters"
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
	Extension           *extension.Extension
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

// shouldRetryTest determines if a failed test should be retried based on retry policies.
// It returns true if the test is eligible for retry, false otherwise.
func shouldRetryTest(ctx context.Context, test *testCase, permittedRetryImageTags []string) bool {
	// Internal tests (no binary) are eligible for retry, we shouldn't really have any of these
	// now that origin is also an extension.
	if test.binary == nil {
		return true
	}

	tlog := logrus.WithField("test", test.name)

	// Test retries were disabled for some suites when they moved to OTE. This exposed small numbers of tests that
	// were actually flaky and nobody knew. We attempted to fix these, a few did not make it in time. Restore
	// retries for specific test names so the overall suite can continue to not retry.
	retryTestNames := []string{
		"[sig-instrumentation] Metrics should grab all metrics from kubelet /metrics/resource endpoint [Suite:openshift/conformance/parallel] [Suite:k8s]", // https://issues.redhat.com/browse/OCPBUGS-57477
		"[sig-network] Services should be rejected for evicted pods (no endpoints exist) [Suite:openshift/conformance/parallel] [Suite:k8s]",               // https://issues.redhat.com/browse/OCPBUGS-57665
		"[sig-node] Pods Extended Pod Container lifecycle evicted pods should be terminal [Suite:openshift/conformance/parallel] [Suite:k8s]",              // https://issues.redhat.com/browse/OCPBUGS-57658
	}
	for _, rtn := range retryTestNames {
		if test.name == rtn {
			tlog.Debug("test has an exception allowing retry")
			return true
		}
	}

	// Get extension info to check if it's from a permitted image
	info, err := test.binary.Info(ctx)
	if err != nil {
		tlog.WithError(err).
			Debug("Failed to get binary info, skipping retry")
		return false
	}

	// Check if the test's source image is in the permitted retry list
	for _, permittedTag := range permittedRetryImageTags {
		if strings.Contains(info.Source.SourceImage, permittedTag) {
			tlog.WithField("image", info.Source.SourceImage).
				Debug("Permitting retry")
			return true
		}
	}

	tlog.WithField("image", info.Source.SourceImage).
		Debug("Test not eligible for retry based on image tag")
	return false
}

func (o *GinkgoRunSuiteOptions) Run(suite *TestSuite, clusterConfig *clusterdiscovery.ClusterConfiguration, junitSuiteName string, monitorTestInfo monitortestframework.MonitorTestInitializationInfo,
	upgrade bool) error {
	ctx := context.Background()
	var sharder Sharder
	switch o.ShardStrategy {
	default:
		sharder = &HashSharder{}
	}

	defaultBinaryParallelism := 10

	// Extract all test binaries
	extractionContext, extractionContextCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer extractionContextCancel()
	cleanUpFn, allBinaries, err := extensions.ExtractAllTestBinaries(extractionContext, defaultBinaryParallelism)
	if err != nil {
		return err
	}
	defer cleanUpFn()

	// Learn about the extension binaries available
	infoContext, infoContextCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer infoContextCancel()
	logrus.Infof("Fetching info from %d extension binaries", len(allBinaries))
	extensionsInfo, err := allBinaries.Info(infoContext, defaultBinaryParallelism)
	if err != nil {
		logrus.Errorf("Failed to fetch extension info: %v", err)
		return fmt.Errorf("failed to fetch extension info: %w", err)
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

	specs, err := allBinaries.ListTests(listContext, defaultBinaryParallelism, envFlags)
	if err != nil {
		return err
	}

	logrus.Infof("Discovered %d total tests", len(specs))

	// Temporarily check for the presence of the [Skipped:xyz] annotation in the test names, once this synthetic test
	// begins to pass we can remove the annotation logic
	var annotatedSkipped []string
	for _, t := range specs {
		if strings.Contains(t.Name, "[Skipped") {
			annotatedSkipped = append(annotatedSkipped, t.Name)
		}
	}
	var fallbackSyntheticTestResult []*junitapi.JUnitTestCase
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

	// skip tests due to newer k8s
	restConfig, err := clusterinfo.GetMonitorRESTConfig()
	if err != nil {
		return err
	}

	// Apply all test filters using the filter chain -- origin previously filtered tests a ton
	// of places, and co-mingled suite, annotation, and cluster state filters in odd ways. This filter
	// chain is the ONLY place tests should be filtered down for determining the final execution set.
	testFilterChain := filters.NewFilterChain(logrus.WithField("component", "test-filter")).
		AddFilter(filters.NewQualifiersFilter(suite.Qualifiers)).
		AddFilter(filters.NewKubeRebaseTestsFilter(restConfig)).
		AddFilter(&filters.DisabledTestsFilter{}).
		AddFilter(filters.NewMatchFnFilter(suite.SuiteMatcher)). // used for file or regexp cli filter on test names
		AddFilter(filters.NewClusterStateFilter(clusterConfig))

	specs, err = testFilterChain.Apply(ctx, specs)
	if err != nil {
		return err
	}

	if len(specs) == 0 {
		return fmt.Errorf("no tests to run")
	}

	tests, err := extensionTestSpecsToOriginTestCases(specs)
	if err != nil {
		return errors.WithMessage(err, "could not convert test specs to origin test cases")
	}

	// this ensures the tests are always run in random order to avoid
	// any intra-tests dependencies
	suiteConfig, _ := ginkgo.GinkgoConfiguration()
	r := rand.New(rand.NewSource(suiteConfig.RandomSeed))
	r.Shuffle(len(tests), func(i, j int) { tests[i], tests[j] = tests[j], tests[i] })

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

	buildsTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-builds]")
	})

	networkTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-network]")
	})

	mustGatherTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-cli] oc adm must-gather")
	})

	logrus.Infof("Found %d openshift tests", len(openshiftTests))
	logrus.Infof("Found %d kube tests", len(kubeTests))
	logrus.Infof("Found %d storage tests", len(storageTests))
	logrus.Infof("Found %d builds tests", len(buildsTests))
	logrus.Infof("Found %d network tests", len(networkTests))
	logrus.Infof("Found %d must-gather tests", len(mustGatherTests))

	// If user specifies a count, duplicate the kube and openshift tests that many times.
	expectedTestCount := len(early) + len(late)
	if count != -1 {
		originalKube := kubeTests
		originalOpenshift := openshiftTests
		originalStorage := storageTests
		originalBuilds := buildsTests
		originalNetwork := networkTests
		originalMustGather := mustGatherTests

		for i := 1; i < count; i++ {
			kubeTests = append(kubeTests, copyTests(originalKube)...)
			openshiftTests = append(openshiftTests, copyTests(originalOpenshift)...)
			storageTests = append(storageTests, copyTests(originalStorage)...)
			buildsTests = append(buildsTests, copyTests(originalBuilds)...)
			networkTests = append(networkTests, copyTests(originalNetwork)...)
			mustGatherTests = append(mustGatherTests, copyTests(originalMustGather)...)
		}
	}
	expectedTestCount += len(openshiftTests) + len(kubeTests) + len(storageTests) + len(buildsTests) + len(networkTests) + len(mustGatherTests)

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

		buildsTestsCopy := copyTests(buildsTests)
		q.Execute(testCtx, buildsTestsCopy, max(1, parallelism/2), testOutputConfig, abortFn) // builds tests only run at half the parallelism, so we can avoid high cpu problems.
		tests = append(tests, buildsTestsCopy...)

		networkTestsCopy := copyTests(networkTests)
		q.Execute(testCtx, networkTestsCopy, max(1, parallelism), testOutputConfig, abortFn) // run network tests separately.
		tests = append(tests, networkTestsCopy...)

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

	// Determine if we should retry any tests for flake detection
	// Don't add more here without discussion with OCP architects, we should be moving towards not having any flakes
	permittedRetryImageTags := []string{"tests"} // tests = openshift-tests image
	if fail > 0 && fail <= suite.MaximumAllowedFlakes {
		var retries []*testCase

		failedUnretriableTestCount := 0
		for _, test := range failing {
			if shouldRetryTest(ctx, test, permittedRetryImageTags) {
				retry := test.Retry()
				retries = append(retries, retry)
				if len(retries) > suite.MaximumAllowedFlakes {
					break
				}
			} else if test.binary != nil {
				// Do not retry extension tests -- we also want to remove retries from origin-sourced
				// tests, but extensions is where we can start.
				failedUnretriableTestCount++
			}
		}

		logrus.Warningf("%d tests failed, %d tests permitted to be retried; %d failures are non-retryable", len(failing), len(retries), failedUnretriableTestCount)

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
			// Explicitly remove flakes from the failing list
			var withoutFlakes []*testCase
		flakeLoop:
			for _, t := range failing {
				for _, f := range flaky {
					if t.name == f {
						continue flakeLoop
					}
				}
				withoutFlakes = append(withoutFlakes, t)
			}
			failing = withoutFlakes

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
				}
				withoutPreconditionFailures = append(withoutPreconditionFailures, t)
			}
			tests = withoutPreconditionFailures

			var failingWithoutPreconditionFailures []*testCase
		failingLoop:
			for _, f := range failing {
				for _, st := range skipped {
					if f.name == st {
						continue failingLoop
					}
				}
				failingWithoutPreconditionFailures = append(failingWithoutPreconditionFailures, f)
			}
			failing = failingWithoutPreconditionFailures
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

	var blockingFailures, informingFailures []*testCase
	for _, test := range failing {
		if isBlockingFailure(test) {
			blockingFailures = append(blockingFailures, test)
		} else {
			test.testOutputBytes = []byte(fmt.Sprintf("*** NON-BLOCKING FAILURE: This test failure is not considered terminal because its lifecycle is '%s' and will not prevent the overall suite from passing.\n\n%s",
				test.extensionTestResult.Lifecycle,
				string(test.testOutputBytes)))
			informingFailures = append(informingFailures, test)
		}
	}

	if len(informingFailures) > 0 {
		names := sets.NewString(testNames(informingFailures)...).List()
		fmt.Fprintf(o.Out, "Informing test failures that don't prevent the overall suite from passing:\n\n\t* %s\n\n", strings.Join(names, "\n\t* "))
	}

	if len(blockingFailures) > 0 {
		names := sets.NewString(testNames(blockingFailures)...).List()
		fmt.Fprintf(o.Out, "Blocking test failures:\n\n\t* %s\n\n", strings.Join(names, "\n\t* "))
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

	switch {
	case len(blockingFailures) > 0:
		return fmt.Errorf("%d blocking fail, %d informing fail, %d pass, %d skip (%s)", len(blockingFailures), len(informingFailures), pass, skip, duration)
	case syntheticFailure:
		return fmt.Errorf("failed because an invariant was violated, %d pass, %d skip (%s)\n", pass, skip, duration)
	case monitorTestResultState != monitor.Succeeded:
		return fmt.Errorf("failed due to a MonitorTest failure")
	case len(informingFailures) > 0:
		fmt.Fprintf(o.Out, "%d informing fail, %d pass, %d skip (%s): suite passes despite failures", len(informingFailures), pass, skip, duration)
	default:
		fmt.Fprintf(o.Out, "%d pass, %d skip (%s)\n", pass, skip, duration)
	}

	return ctx.Err()
}

func isBlockingFailure(test *testCase) bool {
	if test.extensionTestResult == nil {
		return true
	}

	switch test.extensionTestResult.Lifecycle {
	case extensiontests.LifecycleInforming:
		return false
	default:
		return true
	}
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
		AddNetwork(config.NetworkPlugin).
		AddNetworkStack(config.IPFamily).
		AddExternalConnectivity(determineExternalConnectivity(config))

	platform := config.ProviderName
	// MicroShift is defined as "skeleton" in the providerName, but the flag should be "none"
	if platform == "skeleton" {
		platform = "none"
	}
	envFlagBuilder.AddPlatform(platform)

	if config.SingleReplicaTopology {
		// In cases like Microshift, we will not be able to determine the clusterState,
		// so topology will be unset unless we default it properly here
		singleReplicaTopology := configv1.SingleReplicaTopologyMode
		envFlagBuilder.AddTopology(&singleReplicaTopology)
	}

	//Additional flags can only be determined if we are able to obtain the clusterState
	if clusterState != nil {
		envFlagBuilder.AddAPIGroups(clusterState.APIGroups.UnsortedList()...).
			AddFeatureGates(clusterState.EnabledFeatureGates.UnsortedList()...)

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
