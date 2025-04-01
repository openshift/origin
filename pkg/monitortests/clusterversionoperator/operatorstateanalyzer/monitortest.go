package operatorstateanalyzer

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type operatorStateChecker struct {
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &operatorStateChecker{}
}

func (w *operatorStateChecker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *operatorStateChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *operatorStateChecker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*operatorStateChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	ret := monitorapi.Intervals{}
	ret = append(ret, intervalsFromEvents_OperatorAvailable(startingIntervals, nil, beginning, end)...)
	ret = append(ret, intervalsFromEvents_OperatorProgressing(startingIntervals, nil, beginning, end)...)
	ret = append(ret, intervalsFromEvents_OperatorDegraded(startingIntervals, nil, beginning, end)...)

	return ret, nil
}

func (*operatorStateChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*operatorStateChecker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*operatorStateChecker) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
