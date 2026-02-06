package watchpods

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type podWatcher struct {
	kubeClient  kubernetes.Interface
	podInformer coreinformers.PodInformer
}

func NewPodWatcher() monitortestframework.MonitorTest {
	return &podWatcher{}
}

func (w *podWatcher) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *podWatcher) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	w.podInformer = startPodMonitoring(ctx, recorder, w.kubeClient)

	return nil
}

func (w *podWatcher) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// because we are sharing a recorder that we're streaming into, we don't need to have a separate data collection step.
	return nil, nil, nil
}

func (*podWatcher) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	constructedIntervals := monitorapi.Intervals{}
	constructedIntervals = append(constructedIntervals, createPodIntervalsFromInstants(startingIntervals, recordedResources, beginning, end)...)
	constructedIntervals = append(constructedIntervals, intervalsFromEvents_PodChanges(startingIntervals, beginning, end)...)

	return constructedIntervals, nil
}

func (w *podWatcher) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.podInformer == nil {
		return nil, nil
	}

	cacheFailures, err := checkCacheState(ctx, w.kubeClient, w.podInformer)
	if err != nil {
		return nil, fmt.Errorf("error determining cache failures: %w", err)
	}

	testName := "[sig-apimachinery] informers must match live results at the same resource version"
	ret := []*junitapi.JUnitTestCase{}
	if len(cacheFailures) > 0 {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("%s\nabandon all hope", strings.Join(cacheFailures, "\n")),
				},
			},
		)
		// start as a flake
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)
	} else {
		ret = append(ret,
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)
	}

	return ret, nil
}

func (*podWatcher) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*podWatcher) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
