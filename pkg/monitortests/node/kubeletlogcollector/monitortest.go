package kubeletlogcollector

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type kubeletLogCollector struct {
	adminRESTConfig *rest.Config
}

func NewKubeletLogCollector() monitortestframework.MonitorTest {
	return &kubeletLogCollector{}
}

func (w *kubeletLogCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *kubeletLogCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, nil, err
	}
	// MicroShift does not have a proper journal for the node logs api.
	isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
	if err != nil {
		return nil, nil, err
	}
	if isMicroShift {
		return nil, nil, nil
	}

	intervals, err := intervalsFromNodeLogs(ctx, kubeClient, beginning, end)
	return intervals, nil, err
}

func (*kubeletLogCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*kubeletLogCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*kubeletLogCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*kubeletLogCollector) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
