package ginkgo

import (
	"bytes"
	"context"
	"fmt"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/test/extended/util"
	"io/ioutil"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/onsi/ginkgo/v2"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/clioptions/clusterinfo"
	"github.com/openshift/origin/pkg/defaultmonitortests"
	"github.com/openshift/origin/pkg/monitor"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/pkg/monitortestframework"
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

	StartTime time.Time

	ExactMonitorTests   []string
	DisableMonitorTests []string
}

func NewGinkgoRunSuiteOptions(streams genericclioptions.IOStreams) *GinkgoRunSuiteOptions {
	return &GinkgoRunSuiteOptions{
		IOStreams: streams,
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

type RunMatchFunc func(run *RunInformation) bool

type RunInformation struct {
	// jobType may be nil for topologies without
	// platform identification.
	*platformidentification.JobType
	suite *TestSuite
}

type externalBinaryStruct struct {
	// The payload image tag in which an external binary path can be found
	imageTag string
	// The binary path to extract from the image
	binaryPath string

	// nil, nil - run for all suites
	// (), nil, - run for only those matched by include
	// nil, () - run for all except excluded
	// (), () - include overridden by exclude
	includeForRun RunMatchFunc
	excludeForRun RunMatchFunc
}

type externalBinaryResult struct {
	err            error
	skipReason     string
	externalBinary *externalBinaryStruct
	externalTests  []*testCase
}

func (o *GinkgoRunSuiteOptions) Run(suite *TestSuite, junitSuiteName string, monitorTestInfo monitortestframework.MonitorTestInitializationInfo, upgrade bool) error {
	ctx := context.Background()

	tests, err := testsForSuite()
	if err != nil {
		return fmt.Errorf("failed reading origin test suites: %w", err)
	}

	fmt.Fprintf(o.Out, "Found %d tests for in openshift-tests binary for suite %q\n", len(tests), suite.Name)

	var fallbackSyntheticTestResult []*junitapi.JUnitTestCase
	if len(os.Getenv("OPENSHIFT_SKIP_EXTERNAL_TESTS")) == 0 {
		// A registry of available external binaries and in which image
		// they reside in the payload.
		externalBinaries := []externalBinaryStruct{
			{
				imageTag:   "hyperkube",
				binaryPath: "/usr/bin/k8s-tests",
			},
		}

		var (
			externalTests []*testCase
			wg            sync.WaitGroup
			resultCh      = make(chan externalBinaryResult, len(externalBinaries))
			err           error
		)

		// Lines logged to this logger will be included in the junit output for the
		// external binary usage synthetic.
		var extractDetailsBuffer bytes.Buffer
		extractLogger := log.New(&extractDetailsBuffer, "", log.LstdFlags|log.Lmicroseconds)

		oc := util.NewCLIWithoutNamespace("default")
		jobType, err := platformidentification.GetJobType(context.Background(), oc.AdminConfig())
		if err != nil {
			// Microshift does not permit identification. External binaries must
			// tolerate nil jobType.
			extractLogger.Printf("Failed determining job type: %v", err)
		}

		runInformation := &RunInformation{
			JobType: jobType,
			suite:   suite,
		}

		// To extract binaries bearing external tests, we must inspect the release
		// payload under tests as well as extract content from component images
		// referenced by that payload.
		// openshift-tests is frequently run in the context of a CI job, within a pod.
		// CI sets $RELEASE_IMAGE_LATEST to a pullspec for the release payload under test. This
		// pull spec resolve to:
		// 1. A build farm ci-op-* namespace / imagestream location (anonymous access permitted).
		// 2. A quay.io/openshift-release-dev location (for tests against promoted ART payloads -- anonymous access permitted).
		// 3. A registry.ci.openshift.org/ocp-<arch>/release:<tag> (request registry.ci.openshift.org token).
		// Within the pod, we don't necessarily have a pull-secret for #3 OR the component images
		// a payload references (which are private, unless in a ci-op-* imagestream).
		// We try the following options:
		// 1. If set, use the REGISTRY_AUTH_FILE environment variable to an auths file with
		//    pull secrets capable of reading appropriate payload & component image
		//    information.
		// 2. If it exists, use a file /run/secrets/ci.openshift.io/cluster-profile/pull-secret
		//    (conventional location for pull-secret information for CI cluster profile).
		// 3. Use openshift-config secret/pull-secret from the cluster-under-test, if it exists
		//    (Microshift does not).
		// 4. Use unauthenticated access to the payload image and component images.
		registryAuthFilePath := os.Getenv("REGISTRY_AUTH_FILE")

		// if the environment variable is not set, extract the target cluster's
		// platform pull secret.
		if len(registryAuthFilePath) != 0 {
			extractLogger.Printf("Using REGISTRY_AUTH_FILE environment variable: %v", registryAuthFilePath)
		} else {

			// See if the cluster-profile has stored a pull-secret at the conventional location.
			ciProfilePullSecretPath := "/run/secrets/ci.openshift.io/cluster-profile/pull-secret"
			_, err = os.Stat(ciProfilePullSecretPath)
			if !os.IsNotExist(err) {
				extractLogger.Printf("Detected %v; using cluster profile for image access", ciProfilePullSecretPath)
				registryAuthFilePath = ciProfilePullSecretPath
			} else {
				// Inspect the cluster-under-test and read its cluster pull-secret dockerconfigjson value.
				clusterPullSecret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Get(context.Background(), "pull-secret", metav1.GetOptions{})
				if err != nil {
					if kapierrs.IsNotFound(err) {
						extractLogger.Printf("Cluster has no openshift-config secret/pull-secret; falling back to unauthenticated image access")
					} else {
						return fmt.Errorf("unable to read ephemeral cluster pull secret: %w", err)
					}
				} else {
					tmpDir, err := os.MkdirTemp("", "external-binary")
					clusterDockerConfig := clusterPullSecret.Data[".dockerconfigjson"]
					registryAuthFilePath = filepath.Join(tmpDir, ".dockerconfigjson")
					err = os.WriteFile(registryAuthFilePath, clusterDockerConfig, 0600)
					if err != nil {
						return fmt.Errorf("unable to serialize target cluster pull-secret locally: %w", err)
					}

					defer os.Remove(registryAuthFilePath)
					extractLogger.Printf("Using target cluster pull-secrets for registry auth")
				}
			}
		}

		releaseImageReferences, err := extractReleaseImageStream(extractLogger, registryAuthFilePath)
		if err != nil {
			return fmt.Errorf("unable to extract image references from release payload: %w", err)
		}

		for _, externalBinary := range externalBinaries {
			wg.Add(1)
			go func(externalBinary externalBinaryStruct) {
				defer wg.Done()

				var skipReason string
				if (externalBinary.includeForRun != nil && !externalBinary.includeForRun(runInformation)) ||
					(externalBinary.excludeForRun != nil && externalBinary.excludeForRun(runInformation)) {
					skipReason = "excluded by suite selection functions"
				}

				var tagTestSet []*testCase
				var tagErr error
				if len(skipReason) == 0 {
					tagTestSet, tagErr = externalTestsForSuite(ctx, extractLogger, releaseImageReferences, externalBinary.imageTag, externalBinary.binaryPath, registryAuthFilePath)
				}

				resultCh <- externalBinaryResult{
					err:            tagErr,
					skipReason:     skipReason,
					externalBinary: &externalBinary,
					externalTests:  tagTestSet,
				}

			}(externalBinary)
		}

		wg.Wait()
		close(resultCh)

		for result := range resultCh {
			if result.skipReason != "" {
				extractLogger.Printf("Skipping test discovery for image %q and binary %q: %v\n", result.externalBinary.imageTag, result.externalBinary.binaryPath, result.skipReason)
			} else if result.err != nil {
				extractLogger.Printf("Error during test discovery for image %q and binary %q: %v\n", result.externalBinary.imageTag, result.externalBinary.binaryPath, result.err)
				err = result.err
			} else {
				extractLogger.Printf("Discovered %v tests from image %q and binary %q\n", len(result.externalTests), result.externalBinary.imageTag, result.externalBinary.binaryPath)
				externalTests = append(externalTests, result.externalTests...)
			}
		}

		if err == nil {
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
			tests = append(filteredTests, externalTests...)
			extractLogger.Printf("Discovered a total of %v external tests and will run a total of %v\n", len(externalTests), len(tests))
		} else {
			extractLogger.Printf("Errors encountered while extracting one or more external test suites; Falling back to built-in suite: %v\n", err)
			// adding this test twice (one failure here, and success below) will
			// ensure it gets picked as flake further down in synthetic tests processing
			fallbackSyntheticTestResult = append(fallbackSyntheticTestResult, &junitapi.JUnitTestCase{
				Name:      "[sig-arch] External binary usage",
				SystemOut: extractDetailsBuffer.String(),
				FailureOutput: &junitapi.FailureOutput{
					Output: extractDetailsBuffer.String(),
				},
			})
		}
		fmt.Fprintf(o.Out, extractDetailsBuffer.String())
		fallbackSyntheticTestResult = append(fallbackSyntheticTestResult, &junitapi.JUnitTestCase{
			Name:      "[sig-arch] External binary usage",
			SystemOut: extractDetailsBuffer.String(),
		})
	} else {
		fmt.Fprintf(o.Out, "Using built-in tests only due to OPENSHIFT_SKIP_EXTERNAL_TESTS being set\n")
	}

	fmt.Fprintf(o.Out, "Found %d tests (including externals)\n", len(tests))

	// this ensures the tests are always run in random order to avoid
	// any intra-tests dependencies
	suiteConfig, _ := ginkgo.GinkgoConfiguration()
	r := rand.New(rand.NewSource(suiteConfig.RandomSeed))
	r.Shuffle(len(tests), func(i, j int) { tests[i], tests[j] = tests[j], tests[i] })

	tests = suite.Filter(tests)
	if len(tests) == 0 {
		return fmt.Errorf("suite %q does not contain any tests", suite.Name)
	}

	fmt.Fprintf(o.Out, "found %d filtered tests\n", len(tests))

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
		fmt.Fprintf(o.ErrOut, "Interrupted, terminating tests\n")
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
