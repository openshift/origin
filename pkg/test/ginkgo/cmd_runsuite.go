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

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/mod/semver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/dataloader"

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

	// RetryStrategy controls retry behavior and final outcome decisions
	RetryStrategy RetryStrategy

	// WithHypervisorConfigJSON contains JSON configuration for hypervisor-based recovery operations
	WithHypervisorConfigJSON string
}

func NewGinkgoRunSuiteOptions(streams genericclioptions.IOStreams) *GinkgoRunSuiteOptions {
	defaultStrategy, err := createRetryStrategy(defaultRetryStrategy)
	if err != nil {
		panic(fmt.Sprintf("failed to create default retry strategy: %v", err))
	}

	return &GinkgoRunSuiteOptions{
		IOStreams:     streams,
		ShardStrategy: "hash",
		RetryStrategy: defaultStrategy,
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
	availableStrategies := getAvailableRetryStrategies()
	flags.Var(newRetryStrategyFlag(&o.RetryStrategy), "retry-strategy", fmt.Sprintf("Test retry strategy (available: %s, default: %s)", strings.Join(availableStrategies, ", "), defaultRetryStrategy))
	flags.StringVar(&o.WithHypervisorConfigJSON, "with-hypervisor-json", os.Getenv("HYPERVISOR_CONFIG"), "JSON configuration for hypervisor-based recovery operations. Must contain hypervisorIP, sshUser, and privateKeyPath fields.")
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

	// We want to intentionally reduce variability
	// in the ordering to highlight interactions between
	// tests that lead to higher failures
	// we may want to create a set of known seeds
	// and randomly select from that group eventually
	// to compare results
	seed := int64(42)

	// Previous seeding
	// this ensures the tests are always run in random order to avoid
	// any intra-tests dependencies
	// suiteConfig, _ := ginkgo.GinkgoConfiguration()
	// seed := suiteConfig.RandomSeed

	r := rand.New(rand.NewSource(seed))
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

	// start with suite value which should be based on a 3 worker node cluster
	parallelism := suite.Parallelism
	logrus.Infof("Suite defined parallelism %d", parallelism)

	// adjust based on the number of workers
	totalNodes, workerNodes, err := getClusterNodeCounts(ctx, restConfig)
	if err != nil {
		logrus.Errorf("Failed to get cluster node counts: %v", err)
	} else {
		// default to 1/3 the defined parallelism value per worker but use the min of that
		// and the current parallelism value so we don't increase parallelism
		if workerNodes > 0 && parallelism > 0 {
			workerParallelism := max(1, parallelism/3) * workerNodes
			logrus.Infof("Parallelism based on worker node count: %d", workerParallelism)
			parallelism = min(parallelism, workerParallelism)
		}
	}

	// if 0 set our min value
	if parallelism <= 0 {
		parallelism = 10
	}

	// if explicitly set then use the specified value
	if o.Parallelism > 0 {
		parallelism = o.Parallelism
		logrus.Infof("Using specified parallelism value: %d", parallelism)
	}

	logrus.Infof("Total nodes: %d, Worker nodes: %d, Parallelism: %d", totalNodes, workerNodes, parallelism)

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

	// Skip stable cluster check if OPENSHIFT_TESTS_SKIP_STABLE_CLUSTER is set.
	// This is useful in development for rapid iteration where cluster stability
	// verification may be unnecessary and time-consuming.
	var stableClusterTestResults []*junitapi.JUnitTestCase
	if os.Getenv("OPENSHIFT_TESTS_SKIP_STABLE_CLUSTER") == "" {
		logrus.Infof("Waiting for all cluster operators to become stable")
		var err error
		stableClusterTestResults, err = clusterinfo.WaitForStableCluster(ctx, restConfig)
		if err != nil {
			logrus.Errorf("Error waiting for stable cluster: %v", err)
		}
	} else {
		logrus.Infof("Skipping stable cluster check due to OPENSHIFT_TESTS_SKIP_STABLE_CLUSTER environment variable")
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

	storageTests, openshiftTests := splitTests(primaryTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-storage]")
	})

	cliTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-cli]")
	})

	appsTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-apps]")
	})

	nodeTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-node]")
	})

	networkTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-network]")
	})

	buildsTests, openshiftTests := splitTests(openshiftTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-builds]")
	})

	// separate from cliTests
	mustGatherTests, cliTests := splitTests(cliTests, func(t *testCase) bool {
		return strings.Contains(t.name, "[sig-cli] oc adm must-gather")
	})

	logrus.Infof("Found %d openshift tests", len(openshiftTests))
	logrus.Infof("Found %d storage tests", len(storageTests))
	logrus.Infof("Found %d cli tests", len(cliTests))
	logrus.Infof("Found %d apps tests", len(appsTests))
	logrus.Infof("Found %d node tests", len(nodeTests))
	logrus.Infof("Found %d network tests", len(networkTests))
	logrus.Infof("Found %d builds tests", len(buildsTests))
	logrus.Infof("Found %d must-gather tests", len(mustGatherTests))

	// If user specifies a count, duplicate the kube and openshift tests that many times.
	expectedTestCount := len(early) + len(late)
	if count != -1 {
		originalOpenshift := openshiftTests
		originalStorage := storageTests
		originalCLI := cliTests
		originalApps := appsTests
		originalNode := nodeTests
		originalNetwork := networkTests
		originalBuilds := buildsTests
		originalMustGather := mustGatherTests

		for i := 1; i < count; i++ {
			openshiftTests = append(openshiftTests, copyTests(originalOpenshift)...)
			storageTests = append(storageTests, copyTests(originalStorage)...)
			cliTests = append(cliTests, copyTests(originalCLI)...)
			appsTests = append(appsTests, copyTests(originalApps)...)
			nodeTests = append(nodeTests, copyTests(originalNode)...)
			networkTests = append(networkTests, copyTests(originalNetwork)...)
			buildsTests = append(buildsTests, copyTests(originalBuilds)...)
			mustGatherTests = append(mustGatherTests, copyTests(originalMustGather)...)
		}
	}
	expectedTestCount += len(openshiftTests) + len(storageTests) + len(cliTests) + len(appsTests) + len(nodeTests) + len(networkTests) + len(buildsTests) + len(mustGatherTests)

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

		// I thought about randomizing the order of the kube, storage, and openshift tests, but storage dominates our e2e runs, so it doesn't help much.
		storageTestsCopy := copyTests(storageTests)
		q.Execute(testCtx, storageTestsCopy, max(1, parallelism/2), testOutputConfig, abortFn) // storage tests only run at half the parallelism, so we can avoid cloud provider quota problems.
		tests = append(tests, storageTestsCopy...)

		cliTestsCopy := copyTests(cliTests)
		q.Execute(testCtx, cliTestsCopy, max(1, parallelism/2), testOutputConfig, abortFn) // cli tests only run at half the parallelism, so we can avoid high cpu problems.
		tests = append(tests, cliTestsCopy...)

		appsTestsCopy := copyTests(appsTests)
		q.Execute(testCtx, appsTestsCopy, max(1, parallelism/2), testOutputConfig, abortFn) // apps tests only run at half the parallelism, so we can avoid high cpu problems.
		tests = append(tests, appsTestsCopy...)

		nodeTestsCopy := copyTests(nodeTests)
		q.Execute(testCtx, nodeTestsCopy, max(1, parallelism/2), testOutputConfig, abortFn) // run node tests separately at half the parallelism, so we can avoid high cpu problems.
		tests = append(tests, nodeTestsCopy...)

		networkTestsCopy := copyTests(networkTests)
		q.Execute(testCtx, networkTestsCopy, max(1, parallelism/2), testOutputConfig, abortFn) // run network tests separately.
		tests = append(tests, networkTestsCopy...)

		buildsTestsCopy := copyTests(buildsTests)
		q.Execute(testCtx, buildsTestsCopy, max(1, parallelism/2), testOutputConfig, abortFn) // builds tests only run at half the parallelism, so we can avoid high cpu problems.
		tests = append(tests, buildsTestsCopy...)

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

	// Process test retries using the configured retry strategy
	var flaky int
	if o.RetryStrategy.ShouldAttemptRetries(failing, suite) {
		logrus.Infof("Using retry strategy: %s for %d failing tests", o.RetryStrategy.Name(), fail)
		tests, failing, flaky = o.performRetries(testCtx, tests, failing, testRunnerContext, parallelism, testOutputConfig, abortFn)
	} else if fail > 0 {
		logrus.Infof("Retry strategy %s decided not to retry %d failing tests", o.RetryStrategy.Name(), fail)
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

		writeRunSuiteOptions(seed, totalNodes, workerNodes, parallelism, monitorTestInfo, o.JUnitDir, timeSuffix)
	}

	switch {
	case len(blockingFailures) > 0:
		return fmt.Errorf("%d blocking fail, %d informing fail, %d pass, %d flaky, %d skip (%s)", len(blockingFailures), len(informingFailures), pass, flaky, skip, duration)
	case syntheticFailure:
		return fmt.Errorf("failed because an invariant was violated, %d pass, %d flaky, %d skip (%s)", pass, flaky, skip, duration)
	case monitorTestResultState != monitor.Succeeded:
		return fmt.Errorf("failed due to a MonitorTest failure")
	case len(informingFailures) > 0:
		fmt.Fprintf(o.Out, "%d informing fail, %d pass, %d flaky, %d skip (%s): suite passes despite failures", len(informingFailures), pass, flaky, skip, duration)
	default:
		fmt.Fprintf(o.Out, "%d pass, %d flaky, %d skip (%s)\n", pass, flaky, skip, duration)
	}

	return ctx.Err()
}

// performRetries implements retry behavior using the configured RetryStrategy
// to determine retry eligibility, attempt limits, and when to stop retrying.
func (o *GinkgoRunSuiteOptions) performRetries(ctx context.Context, tests []*testCase, failing []*testCase, testRunnerContext *commandContext, parallelism int, testOutputConfig testOutputConfig, abortFn testAbortFunc) ([]*testCase, []*testCase, int) {
	// Track attempts per test name
	testAttempts := make(map[string][]*testCase)

	// Initialize with original failed tests, checking strategy eligibility
	for _, test := range failing {
		maxRetries := o.RetryStrategy.GetMaxRetries(test)
		if maxRetries > 0 {
			testAttempts[test.name] = []*testCase{test}
			logrus.Infof("Test %s eligible for up to %d retries", test.name, maxRetries)
		} else {
			logrus.Warningf("Test %s not eligible for retries (strategy returned 0)", test.name)
		}
	}

	logrus.Infof("Starting retries for %d eligible tests", len(testAttempts))

	q := newParallelTestQueue(testRunnerContext)

	// Track which tests should no longer be retried
	completedTests := sets.New[string]()

	// Perform retry attempts using strategy to control retry behavior
	for len(testAttempts) > len(completedTests) {
		var retries []*testCase

		// Check each test to see if it should continue retrying
		for testName, attempts := range testAttempts {
			// Skip tests that are already completed
			if completedTests.Has(testName) {
				continue
			}

			originalFailure := attempts[0]
			lastAttempt := attempts[len(attempts)-1]
			nextAttemptNumber := len(attempts) + 1

			if o.RetryStrategy.ShouldContinue(originalFailure, attempts, nextAttemptNumber) {
				retry := lastAttempt.Retry()
				retries = append(retries, retry)
			} else {
				// Strategy says stop retrying this test, but keep it in testAttempts for final processing
				completedTests.Insert(testName)
				logrus.Infof("Strategy decided to stop retrying test %s after %d attempts", testName, len(attempts))
			}
		}

		if len(retries) == 0 {
			break
		}

		logrus.Infof("Retrying %d tests", len(retries))

		// Execute retries
		q.Execute(ctx, retries, parallelism, testOutputConfig, abortFn)

		// Process results and update attempts
		for _, retry := range retries {
			if retry.flake {
				// Skip flaky retries, keep original failure
				fmt.Fprintf(o.Out, "Ignoring retry that returned a flake, original failure is authoritative for test: %s\n", retry.name)
				continue
			}

			testAttempts[retry.name] = append(testAttempts[retry.name], retry)
			// Don't add individual retry attempts to tests list yet - we'll decide later based on strategy outcome
		}
	}

	// Process final results
	var finalFlaky []string
	var finalSkipped []string
	var stillFailing []*testCase

	for testName, attempts := range testAttempts {
		// Use the retry strategy to determine the outcome
		outcome := o.RetryStrategy.DecideOutcome(attempts)

		switch outcome {
		case RetryOutcomeSkipped:
			finalSkipped = append(finalSkipped, testName)

		case RetryOutcomeFlaky:
			// Consider it flaky - add ALL attempts to tests slice
			finalFlaky = append(finalFlaky, testName)
			// Add all retry attempts to tests slice (excluding original which is already there)
			for _, attempt := range attempts[1:] {
				if !attempt.flake {
					tests = append(tests, attempt)
				}
			}

		case RetryOutcomeFail:
			rollupTest := o.createSingleFailureRollupTest(testName, attempts)

			// Replace original failed test with rollup test
			for i, t := range tests {
				if t.name == testName {
					tests[i] = rollupTest
					break
				}
			}
			stillFailing = append(stillFailing, rollupTest)
		}
	}

	// Remove flaky tests from failing list
	if len(finalFlaky) > 0 {
		var withoutFlakes []*testCase
	flakeLoop:
		for _, t := range failing {
			for _, f := range finalFlaky {
				if t.name == f {
					continue flakeLoop
				}
			}
			withoutFlakes = append(withoutFlakes, t)
		}
		failing = withoutFlakes
		sort.Strings(finalFlaky)
		fmt.Fprintf(o.Out, "Flaky tests:\n\n%s\n\n", strings.Join(finalFlaky, "\n"))
	}
	failing = append(failing, stillFailing...)

	// Write retry statistics to autodl file for any retry strategy (except none)
	if o.RetryStrategy.Name() != "none" && len(o.JUnitDir) > 0 {
		if err := writeRetryStatistics(testAttempts, o.RetryStrategy, o.JUnitDir); err != nil {
			logrus.WithError(err).Error("Failed to write retry statistics autodl file")
		}
	}

	// Handle skipped tests
	if len(finalSkipped) > 0 {
		var withoutPreconditionFailures []*testCase
	testLoop:
		for _, t := range tests {
			for _, st := range finalSkipped {
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
			for _, st := range finalSkipped {
				if f.name == st {
					continue failingLoop
				}
			}
			failingWithoutPreconditionFailures = append(failingWithoutPreconditionFailures, f)
		}
		failing = failingWithoutPreconditionFailures

		sort.Strings(finalSkipped)
		fmt.Fprintf(o.Out, "Skipped tests that failed a precondition:\n\n%s\n\n", strings.Join(finalSkipped, "\n"))
	}

	return tests, failing, len(finalFlaky)
}

// createSingleFailureRollupTest creates a rollup test case combining all retry attempts. This is needed to produce a single failure
// artifact, otherwise our systems would consider it a flake.
func (o *GinkgoRunSuiteOptions) createSingleFailureRollupTest(testName string, attempts []*testCase) *testCase {
	var combinedOutput strings.Builder

	successCount := 0
	failureCount := 0
	for _, attempt := range attempts {
		if attempt.failed {
			failureCount++
		}
		if attempt.success {
			successCount++
		}
	}

	if successCount > 0 {
		combinedOutput.WriteString(fmt.Sprintf("Test '%s' failed %d out of %d attempts but had some successes (retry strategy marked as failure).\n\n",
			testName, failureCount, len(attempts)))
	} else {
		combinedOutput.WriteString(fmt.Sprintf("Test '%s' failed all %d attempts.\n\n",
			testName, len(attempts)))
	}

	for i, attempt := range attempts {
		status := "FAILED"
		if attempt.success {
			status = "PASSED"
		} else if attempt.skipped {
			status = "SKIPPED"
		}

		combinedOutput.WriteString(fmt.Sprintf("=== Attempt %d: %s ===\n", i+1, status))
		combinedOutput.Write(attempt.testOutputBytes)
		combinedOutput.WriteString("\n\n")
	}

	// Create rollup test case based on the first attempt
	rollupTest := *attempts[0]
	rollupTest.testOutputBytes = []byte(combinedOutput.String())
	rollupTest.failed = true
	rollupTest.success = false
	rollupTest.skipped = false

	return &rollupTest
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

func writeRunSuiteOptions(seed int64, totalNodes, workerNodes, parallelism int, info monitortestframework.MonitorTestInitializationInfo, artifactDir, timeSuffix string) {
	var rows []map[string]string

	rows = make([]map[string]string, 0)
	rows = append(rows, map[string]string{"RandomSeed": fmt.Sprintf("%d", seed), "ClusterStability": string(info.ClusterStabilityDuringTest),
		"WorkerNodes": fmt.Sprintf("%d", workerNodes), "TotalNodes": fmt.Sprintf("%d", totalNodes), "Parallelism": fmt.Sprintf("%d", parallelism)})
	dataFile := dataloader.DataFile{
		TableName: "run_suite_options",
		Schema: map[string]dataloader.DataType{"ClusterStability": dataloader.DataTypeString, "RandomSeed": dataloader.DataTypeInteger, "WorkerNodes": dataloader.DataTypeInteger,
			"TotalNodes": dataloader.DataTypeInteger, "Parallelism": dataloader.DataTypeInteger},
		Rows: rows,
	}
	fileName := filepath.Join(artifactDir, fmt.Sprintf("run-suite-options%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))
	err := dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		logrus.WithError(err).Warnf("unable to write data file: %s", fileName)
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

	// Additional flags can only be determined if we are able to obtain the clusterState
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
			// TODO(sgoeddel): eventually, we may need to check every node and pass "multi" as the value if any of them differ from the masters
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

func getClusterNodeCounts(ctx context.Context, config *rest.Config) (int, int, error) {
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return 0, 0, err
	}

	totalNodes := 0
	workerNodes := 0

	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
	if err != nil {
		return 0, 0, err
	}

	workerNodes = len(nodes.Items)
	logrus.Infof("Found %d worker nodes", workerNodes)

	nodes, err = kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, 0, err
	}

	totalNodes = len(nodes.Items)
	logrus.Infof("Found %d nodes", totalNodes)

	return totalNodes, workerNodes, nil
}
