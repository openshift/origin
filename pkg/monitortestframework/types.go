package monitortestframework

import (
	"context"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type ClusterStabilityDuringTest string

var (
	// Stable means that at no point during testing do we expect a component to take downtime and upgrades are not happening.
	Stable ClusterStabilityDuringTest = "Stable"
	// TODO only bring this back if we have some reason to collect Upgrade specific information.  I can't think of reason.
	// TODO please contact @deads2k for vetting if you think you found something
	// Upgrade    ClusterStabilityDuringTest = "Upgrade"
	// Disruptive means that the suite is expected to induce outages to the cluster.
	// Some of the more sensitive monitor tests have their junit results converted to flakes
	// so that failures caused by intentional disruption are visible but cannot cause job failures.
	// Not all monitor tests are flaked — critical invariants still produce
	// hard failures.
	Disruptive ClusterStabilityDuringTest = "Disruptive"
	// SpotCheck means the job is a minimal, less-sensitive spot-check run. A subset of monitor
	// tests run, and some of the more sensitive ones have their junit results converted to flakes
	// so failures are visible but cannot cause job failures. Critical invariants still produce
	// hard failures.
	SpotCheck ClusterStabilityDuringTest = "SpotCheck"
)

type MonitorTestInitializationInfo struct {
	ClusterStabilityDuringTest ClusterStabilityDuringTest
	// UpgradeTargetImage is only set for upgrades.  It is set to the *final* destination version.
	UpgradeTargetPayloadImagePullSpec string

	// ExactMonitorTests will filter the available monitor tests down to only those contained in the provided list
	ExactMonitorTests []string

	// DisableMonitorTests will remove any monitor tests contained in the provided list
	DisableMonitorTests []string

	// SuiteName is the name of the test suite being run (e.g. "kubernetes/conformance", "openshift/conformance/parallel").
	SuiteName string
}

// FlakeJunits controls whether a monitor test converts its junit results to flakes.
// When true, every failure gets an additional pass entry appended so that it appears
// as a flake — visible in CI results but unable to cause a job to fail.
type FlakeJunits bool

const (
	// HardFail means junit results are reported as-is: failures will fail the job.
	HardFail FlakeJunits = false
	// AsFlake means junit failures are converted to flakes: visible but unable to fail the job.
	AsFlake FlakeJunits = true
)

// JUnitsToFlakes converts a slice of junit results so that every test name that has a failure
// also has a corresponding pass entry. This makes all failures appear as flakes — visible in CI
// results but unable to cause a job to fail. Test names that already have only pass entries are
// left unchanged.
func JUnitsToFlakes(junits []*junitapi.JUnitTestCase) []*junitapi.JUnitTestCase {
	if len(junits) == 0 {
		return junits
	}

	// Collect the set of test names that have a failure but no existing pass.
	hasPass := map[string]bool{}
	hasFail := map[string]bool{}
	for _, j := range junits {
		if j == nil {
			continue
		}
		if j.FailureOutput != nil {
			hasFail[j.Name] = true
		} else if j.SkipMessage == nil {
			// It's a pass (not a skip, not a failure).
			hasPass[j.Name] = true
		}
	}

	// For every test name that failed without a pass, append a pass to make it a flake.
	// Sort the names so the appended entries have a deterministic order across runs.
	failNames := make([]string, 0, len(hasFail))
	for name := range hasFail {
		if !hasPass[name] {
			failNames = append(failNames, name)
		}
	}
	if len(failNames) == 0 {
		return junits
	}
	sort.Strings(failNames)

	// Build a new slice so callers that retain a reference to the original are
	// not surprised by in-place mutation of the backing array.
	out := make([]*junitapi.JUnitTestCase, len(junits), len(junits)+len(failNames))
	copy(out, junits)
	for _, name := range failNames {
		out = append(out, &junitapi.JUnitTestCase{Name: name})
	}

	return out
}

type OpenshiftTestImageGetterFunc func(ctx context.Context, adminRESTConfig *rest.Config) (imagePullSpec string, notSupportedReason string, err error)

type MonitorTest interface {
	// PrepareCollection is responsible for setting up all resources required for collection of data on the cluster
	// and returning when preparation is complete.
	// An error will not stop execution, but will cause a junit failure that will cause the job run to fail.
	// This allows us to know when setups fail.
	PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error

	// StartCollection is responsible for collection of data on the cluster and may block for the until the context is cancelled.
	// An error will not stop execution, but will cause a junit failure that will cause the job run to fail.
	// This allows us to know when setups fail.
	StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error

	// CollectData will only be called once near the end of execution, before all Intervals are inspected.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	// storageDir is for gathering data only, not for writing in this stage.  To store data, use WriteContentToStorage.
	// The returned JUnitTestCases should be stable in different runs as the test pass/fail rate
	// is calculated in an aggregated to improve CI signal. I.e., if a JUnitTestCase shows up in
	// some run, then it should stay in other runs as well.
	// See https://docs.ci.openshift.org/docs/release-oversight/improving-ci-signal/#passfail-rates-for-running-jobs-10-times
	// for details.
	// In addition, we should avoid renaming a JUnitTestCase, e.g., by not using
	// any specific numbers that could be changed over time.
	// See https://github.com/openshift-eng/ci-test-mapping?tab=readme-ov-file#renaming-tests
	// for details.
	CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error)

	// ConstructComputedIntervals is called after all InvariantTests have produced raw Intervals.
	// Order of ConstructComputedIntervals across different InvariantTests is not guaranteed.
	// Return *only* the constructed intervals.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (constructedIntervals monitorapi.Intervals, err error)

	// EvaluateTestsFromConstructedIntervals is called after all Intervals are known and can produce
	// junit tests for reporting purposes.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	// The returned JUnitTestCases should be stable in different runs as the test pass/fail rate
	// is calculated in an aggregated to improve CI signal. I.e., if a JUnitTestCase shows up in
	// some run, then it should stay in other runs as well.
	// See https://docs.ci.openshift.org/docs/release-oversight/improving-ci-signal/#passfail-rates-for-running-jobs-10-times
	// for details.
	// In addition, we should avoid renaming a JUnitTestCase, e.g., by not using
	// any specific numbers that could be changed over time.
	// See https://github.com/openshift-eng/ci-test-mapping?tab=readme-ov-file#renaming-tests
	// for details.
	EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error)

	// WriteContentToStorage writes content to the storage directory that is collected by openshift CI.
	// Do not write junits, intervals, or tracked resources.
	// 1. junits.  Those should be returned from EvaluateTestsFromConstructedIntervals
	// 2. intervals.  Those should be returned from CollectData and ConstructComputedIntervals
	// 3. tracked resources.  Those are written by some default monitorTests.
	// You *may* choose to store state in CollectData that you later persist via this method. An example might be
	// code that scans audit logs and reports summaries of top actors.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error

	// Cleanup must be idempotent and it may be called multiple times in any scenario.  Multiple defers, multi-registered
	// abort handlers, abort handler running concurrent to planned shutdown.  Make your cleanup callable multiple times.
	// Errors reported will cause job runs to fail to ensure cleanup functions work reliably.
	Cleanup(ctx context.Context) error
}

type MonitorTestRegistry interface {
	AddRegistryOrDie(registry MonitorTestRegistry)

	// AddMonitorTest adds an invariant test with a particular name, the name will be used to create a testsuite.
	// The jira component will be forced into every JunitTestCase.
	AddMonitorTest(name, jiraComponent string, monitorTest MonitorTest) error

	AddMonitorTestOrDie(name, jiraComponent string, monitorTest MonitorTest)

	GetRegistryFor(names ...string) (MonitorTestRegistry, error)
	ListMonitorTests() sets.String

	// PrepareCollection is responsible for setting up all resources required for collection of data on the cluster
	// and returning when preparation is complete.
	// An error will not stop execution, but will cause a junit failure that will cause the job run to fail.
	// This allows us to know when setups fail.
	PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) ([]*junitapi.JUnitTestCase, error)

	// StartCollection is responsible for setting up all resources required for collection of data on the cluster.
	// An error will not stop execution, but will cause a junit failure that will cause the job run to fail.
	// This allows us to know when setups fail.
	StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) ([]*junitapi.JUnitTestCase, error)

	// CollectData will only be called once near the end of execution, before all Intervals are inspected.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error)

	// ConstructComputedIntervals is called after all InvariantTests have produced raw Intervals.
	// Order of ConstructComputedIntervals across different InvariantTests is not guaranteed.
	// Return *only* the constructed intervals.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error)

	// EvaluateTestsFromConstructedIntervals is called after all Intervals are known and can produce
	// junit tests for reporting purposes.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error)

	// WriteContentToStorage writes content to the storage directory that is collected by openshift CI.
	// Do not write.
	// 1. junits.  Those should be returned from EvaluateTestsFromConstructedIntervals
	// 2. intervals.  Those should be returned from CollectData and ConstructComputedIntervals
	// 3. tracked resources.  Those are written by some default monitorTests.
	// You *may* choose to store state in CollectData that you later persist via this method. An example might be
	// code that scans audit logs and reports summaries of top actors.
	WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) ([]*junitapi.JUnitTestCase, error)

	// Cleanup must be idempotent and it may be called multiple times in any scenario.  Multiple defers, multi-registered
	// abort handlers, abort handler running concurrent to planned shutdown.  Make your cleanup callable multiple times.
	// Errors reported will cause job runs to fail to ensure cleanup functions work reliably.
	Cleanup(ctx context.Context) ([]*junitapi.JUnitTestCase, error)

	getMonitorTests() map[string]*monitorTesttItem
}
