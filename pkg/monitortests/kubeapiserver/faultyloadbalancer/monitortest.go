package faultyloadbalancer

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	MonitorName = "faulty-load-balancer"
)

var (
	// these are the load balancer types we are interested in
	lbTypes = map[string]struct{}{
		"internal-lb":     {},
		"external-lb":     {},
		"service-network": {},
	}
)

// NewMonitorTest returns a monitor test that iterates through the constructed
// intervals {APIServerGracefulShutdown|APIUnreachableFromClient} and detects
// a faulty load balancer
//
// a) APIServerGracefulShutdown: represnts an interval while a kube-apiserver
// instance is going through a graceful termination process
// b) APIUnreachableFromClient: represnts an interval while client experiences
// connectivity errors to the kube-apiserver
//
// This monitor test goes through each kube-apiserver shutdoen interval and
// finds any overlapping APIUnreachableFromClient interval, this implies that
// new or reused connections to the kube-apiserver over a certain load balancer
// (external, inernal, or service network) were not handled gracefully while a
// kube-apiserver instance was rolling out.
func NewMonitorTest() monitortestframework.MonitorTest {
	return &monitorTest{}
}

type monitorTest struct {
	monitor *faultyLBMonitor
}

func (test *monitorTest) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (test *monitorTest) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	test.monitor = &faultyLBMonitor{}
	framework.Logf("monitor[%s]: initialized", MonitorName)
	return nil
}

func (test *monitorTest) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return monitorapi.Intervals{}, nil, nil
}

func (*monitorTest) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (test *monitorTest) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junitTest := &junitTest{name: "[sig-apimachinery] new and reused connections to kube-apiserver should be handled gracefully during the graceful termination process"}

	intervals, shutdownIntervalCount := test.monitor.Filter(finalIntervals)
	framework.Logf("monitor[%s]: found %d interesting intervals, kube-apiserver shutdown interval count: %d", MonitorName, len(intervals), shutdownIntervalCount)

	// the following constraints define pass/fail for this test:
	// a) if we don't find any valid kube-apiserver shutdown interval, then
	// this test is a noop, so we mark the test as skipped
	// b) we find at least one valid kube-apiserver shutdown interval, but no
	// overlapping client error interval, this test is a pass
	// c) we find at least one valid kube-apiserver shutdown interval, and at
	// least one overlapping client error interval, this test is a flake
	if len(intervals) == 0 || shutdownIntervalCount <= 0 {
		// a) no kube-apisever shutdown interval seen, mark the test as skipped
		return junitTest.Skip(), nil
	}

	test.monitor.Check(intervals, junitTest.OnOverlap)
	// we handle b, or c here
	return junitTest.Result(), nil
}

func (*monitorTest) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*monitorTest) Cleanup(ctx context.Context) error {
	return nil
}

type junitTest struct {
	name   string
	failed *junitapi.JUnitTestCase
}

// OnOverlap is called when a kube-apiserver shutdown interval overlaps with an
// API unreachable from client (APIUnreachableFromClient) interval, this function
// constructs the junit test case by taking into account each overlap
func (jut *junitTest) OnOverlap(shutdown, unreachable monitorapi.Interval) {
	if jut.failed == nil {
		jut.failed = &junitapi.JUnitTestCase{
			Name:          jut.name,
			SystemOut:     fmt.Sprintf("faulty load balancer detected"),
			FailureOutput: &junitapi.FailureOutput{},
		}
	}

	lbType := unreachable.Condition.Locator.Keys[monitorapi.LocatorAPIUnreachableHostKey]
	msg := fmt.Sprintf("client observed connection error during kube-apiserver rollout, type: %s\nkube-apiserver: %s\nclient: %s\n", lbType, shutdown.String(), unreachable.String())
	jut.failed.FailureOutput.Output = fmt.Sprintf("%s\n%s", jut.failed.FailureOutput.Output, msg)
}

func (jut *junitTest) Skip() []*junitapi.JUnitTestCase {
	passed := &junitapi.JUnitTestCase{
		Name:      jut.name,
		SystemOut: "No kube-apiserver shutdown interval found",
	}
	return []*junitapi.JUnitTestCase{passed}
}

func (jut *junitTest) Result() []*junitapi.JUnitTestCase {
	passed := &junitapi.JUnitTestCase{
		Name:      jut.name,
		SystemOut: "",
	}

	if jut.failed != nil {
		// TODO: we will use sippy to find occurrences of faulty load
		// balancers where it flakes. Once we know it's fully passing then
		// we can remove the flake test case.
		return []*junitapi.JUnitTestCase{jut.failed, passed}
	}
	return []*junitapi.JUnitTestCase{passed}
}

// faultyLBMonitor iterates through the APIServerGracefulShutdown and APIUnreachableFromClient
// intervals, and detects if there is an APIUnreachableFromClient interval that overlaps with
// a kube-apiserver shutdoen interval, this is indicative of a network with a faulty load balancer
type faultyLBMonitor struct {
	// the last kube-apiserver shutdown interval seen, we assume that there
	// will be no two kube-apiserver shutdown intervals that overlap each other.
	shutdown *monitorapi.Interval
}

func (flb *faultyLBMonitor) Filter(allIntervals monitorapi.Intervals) (monitorapi.Intervals, int) {
	intervals := make([]monitorapi.Interval, 0)
	var shutdownIntervalCount int
	for i := range allIntervals {
		interval := allIntervals[i]
		switch {
		case interval.Source == monitorapi.APIServerGracefulShutdown:
			// we are interested only in closed kube-apiserver shutdown interval
			// TODO: we have a known bug where kubelet prematurely terminates the
			// kube-apiserver container, see https://issues.redhat.com/browse/OCPBUGS-38381,
			// or the TerminationGracefulTerminationFinished event is not written
			// successfully to the storage for some unlikely reason. we want to avoid
			// an open APIServerGracefulShutdown interval
			if server, ok := interval.Locator.Keys[monitorapi.LocatorServerKey]; ok && server == "kube-apiserver" && !interval.To.IsZero() {
				intervals = append(intervals, interval)
				shutdownIntervalCount++
			}
		case interval.Source == monitorapi.SourceAPIUnreachableFromClient:
			// we take APIUnreachableFromClient intervals that has/ the desired
			// load balancer type: internal, external, or the service network
			if host, ok := interval.Locator.Keys[monitorapi.LocatorAPIUnreachableHostKey]; ok {
				if _, ok := lbTypes[host]; ok {
					intervals = append(intervals, interval)
				}
			}
		}
	}
	return intervals, shutdownIntervalCount
}

// Check invokes the callback function with the overlapping kube-apiserver
// shutdown interval and the APIUnreachableFromClient interval.
// The given intervals from source {APIServerGracefulShutdown|APIUnreachableFromClient}
func (flb *faultyLBMonitor) Check(intervals monitorapi.Intervals, overlapFn func(shutdown, unreachable monitorapi.Interval)) {
	sort.Sort(bySource(intervals))

	for i := range intervals {
		interval := intervals[i]
		if interval.Source == monitorapi.APIServerGracefulShutdown {
			flb.shutdown = &interval
			continue
		}
		if flb.shutdown == nil {
			continue
		}

		// at this point, we have a preceding kube-apiserver shutdown interval,
		// and we are interested in an APIUnreachableFromClient interval that
		// starts while the given kube-apiserver shutdown interval was in progress
		if flb.shutdown.To.After(interval.From) {
			overlapFn(*flb.shutdown, interval)
		}
	}
}

// to sort a list of intervals that consist of source = {APIServerGracefulShutdown|APIUnreachableFromClient}
type bySource []monitorapi.Interval

func (intervals bySource) Less(i, j int) bool {
	switch d := intervals[i].From.Sub(intervals[j].From); {
	case d < 0:
		return true
	case d > 0:
		return false
	}

	// we have a tie, in this case we want the APIServerGracefulShutdown
	// interval to appear before in the sorted list
	switch {
	case intervals[i].Source == monitorapi.APIServerGracefulShutdown:
		return true
	case intervals[j].Source == monitorapi.APIServerGracefulShutdown:
		return false
	}

	return true
}
func (intervals bySource) Len() int { return len(intervals) }
func (intervals bySource) Swap(i, j int) {
	intervals[i], intervals[j] = intervals[j], intervals[i]
}
