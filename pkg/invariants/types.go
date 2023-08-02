package invariants

import (
	"context"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type InvariantTest interface {
	// StartCollection is responsible for setting up all resources required for collection of data on the cluster.
	// An error will not stop execution, but will cause a junit failure that will cause the job run to fail.
	// This allows us to know when setups fail.
	StartCollection(ctx context.Context, adminRESTConfig *rest.Config) error

	// CollectData will only be called once near the end of execution, before all Intervals are inspected.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	CollectData(ctx context.Context) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error)

	// ConstructComputedIntervals is called after all InvariantTests have produced raw Intervals.
	// Order of ConstructComputedIntervals across different InvariantTests is not guaranteed.
	// Return *only* the constructed intervals.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals) (constructedIntervals monitorapi.Intervals, err error)

	// EvaluateTestsFromConstructedIntervals is called after all Intervals are known and can produce
	// junit tests for reporting purposes.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error)

	// Cleanup must be idempotent and it may be called multiple times in any scenario.  Multiple defers, multi-registered
	// abort handlers, abort handler running concurrent to planned shutdown.  Make your cleanup callable multiple times.
	// Errors reported will cause job runs to fail to ensure cleanup functions work reliably.
	Cleanup(ctx context.Context) error
}

type InvariantRegistry interface {
	// AddInvariant adds an invariant test with a particular name, the name will be used to create a testsuite.
	// The jira component will be forced into every JunitTestCase.
	AddInvariant(name, jiraComponent string, invariantTest InvariantTest) error

	AddInvariantOrDie(name, jiraComponent string, invariantTest InvariantTest)

	// StartCollection is responsible for setting up all resources required for collection of data on the cluster.
	// An error will not stop execution, but will cause a junit failure that will cause the job run to fail.
	// This allows us to know when setups fail.
	StartCollection(ctx context.Context, adminRESTConfig *rest.Config) ([]*junitapi.JUnitTestCase, error)

	// CollectData will only be called once near the end of execution, before all Intervals are inspected.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	CollectData(ctx context.Context) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error)

	// ConstructComputedIntervals is called after all InvariantTests have produced raw Intervals.
	// Order of ConstructComputedIntervals across different InvariantTests is not guaranteed.
	// Return *only* the constructed intervals.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error)

	// EvaluateTestsFromConstructedIntervals is called after all Intervals are known and can produce
	// junit tests for reporting purposes.
	// Errors reported will be indicated as junit test failure and will cause job runs to fail.
	EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error)

	// Cleanup must be idempotent and it may be called multiple times in any scenario.  Multiple defers, multi-registered
	// abort handlers, abort handler running concurrent to planned shutdown.  Make your cleanup callable multiple times.
	// Errors reported will cause job runs to fail to ensure cleanup functions work reliably.
	Cleanup(ctx context.Context) ([]*junitapi.JUnitTestCase, error)
}
