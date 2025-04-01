package nodestateanalyzer

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type nodeStateAnalyzer struct {
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &nodeStateAnalyzer{}
}

func (w *nodeStateAnalyzer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *nodeStateAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *nodeStateAnalyzer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*nodeStateAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	ret := monitorapi.Intervals{}
	ret = append(ret, intervalsFromEvents_NodeChanges(startingIntervals, nil, beginning, end)...)

	return ret, nil
}

func (*nodeStateAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*nodeStateAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*nodeStateAnalyzer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
