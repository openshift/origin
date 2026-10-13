package staticpodinstall

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"

	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	MonitorName = "staicpod-install-monitor"
)

func NewStaticPodInstallMonitorTest() *monitorTest {
	analyzers := []analyzer{
		&staticPodReadinessProbeEventAnalyzer{
			windowsByPod: map[podKey][]*podWindow{},
		},
		&installerPodPLEGEventAnalyzer{
			windows: map[podKey]*podWindow{},
		},
	}
	return &monitorTest{analyzers: analyzers}
}

type monitorTest struct {
	analyzers []analyzer
	computed  monitorapi.Intervals
}

func (mt *monitorTest) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (mt *monitorTest) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (mt *monitorTest) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (mt *monitorTest) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	mt.computed = mt.construct(startingIntervals)

	return mt.computed, nil
}

func (mt *monitorTest) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junitTest := &junitTest{
		name:     "[sig-apimachinery] installer pods should not run concurrently on two or more nodes",
		computed: mt.computed,
	}

	framework.Logf("monitor[%s]: found %d intervals of interest", MonitorName, len(junitTest.computed))

	// the following constraints define pass/fail for this test:
	// a) if we don't find any constructed/computed interval, then
	// this test is a noop, so we mark the test as skipped
	// b) we find constructed/computed intervals, but no occurrences of
	// concurrent situation, this test is a pass
	// c) otherwise, there is at least one incident of a
	// concurrent situation, this test is a flake/fail
	if len(junitTest.computed) == 0 {
		// a) no constructed/computed interval observed, mark the test as skipped
		return junitTest.Skip(), nil
	}

	// now check if there are any occurrences of concurrent interval
	// and return either b,  or c
	return junitTest.Result(), nil
}

func (*monitorTest) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*monitorTest) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

type analyzer interface {
	// want should return true if this analyzer is interested in this interval
	want(interval monitorapi.Interval) bool
	// analyze handles or processes the wanted interval
	analyze(interval monitorapi.Interval)
	// result returns a set of computed or constructed intervals
	result() monitorapi.Intervals
}

func (mt *monitorTest) construct(starting monitorapi.Intervals) monitorapi.Intervals {
	for _, interval := range starting {
		for _, analyzer := range mt.analyzers {
			if analyzer.want(interval) {
				analyzer.analyze(interval)
			}
		}
	}

	computed := monitorapi.Intervals{}
	for _, analyzer := range mt.analyzers {
		computed = append(computed, analyzer.result()...)
	}
	return computed
}

type junitTest struct {
	name     string
	computed monitorapi.Intervals
}

func (jut *junitTest) Result() []*junitapi.JUnitTestCase {
	passed := &junitapi.JUnitTestCase{
		Name:      jut.name,
		SystemOut: "",
	}

	concurrent := monitorapi.Intervals{}
	for _, interval := range jut.computed {
		if node, ok := interval.Message.Annotations["concurrent-node"]; ok && len(node) > 0 {
			concurrent = append(concurrent, interval)
		}
	}

	if len(concurrent) == 0 {
		// passed
		return []*junitapi.JUnitTestCase{passed}
	}

	// flake
	failed := &junitapi.JUnitTestCase{
		Name:          jut.name,
		SystemOut:     fmt.Sprintf("installer pods running concurrently on two or more nodes"),
		FailureOutput: &junitapi.FailureOutput{},
	}
	for _, interval := range concurrent {
		failed.FailureOutput.Output = fmt.Sprintf("%s\n%s", failed.FailureOutput.Output, interval.String())
	}

	// TODO: for now, we flake the test, Once we know it's fully
	// passing then we can remove the flake test case.
	return []*junitapi.JUnitTestCase{failed, passed}
}

func (jut *junitTest) Skip() []*junitapi.JUnitTestCase {
	skipped := &junitapi.JUnitTestCase{
		Name: jut.name,
		SkipMessage: &junitapi.SkipMessage{
			Message: "No intervals of interest seen",
		},
	}
	return []*junitapi.JUnitTestCase{skipped}
}
