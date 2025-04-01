package timelineserializer

import (
	"context"
	"sort"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type timelineSerializer struct {
}

func NewTimelineSerializer() monitortestframework.MonitorTest {
	return &timelineSerializer{}
}

func (w *timelineSerializer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *timelineSerializer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *timelineSerializer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*timelineSerializer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*timelineSerializer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*timelineSerializer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	errs := []error{}
	var err error

	// use custom sorting here so that we can prioritize the sort order to make the intervals html page as readable
	// as possible. This makes the events *not* sorted by time.
	customOrderedEvents := make([]monitorapi.Interval, len(finalIntervals))
	for i := range finalIntervals {
		customOrderedEvents[i] = finalIntervals[i]
	}
	sort.Stable(monitorapi.ByTimeWithNamespacedPods(customOrderedEvents))

	// these produce the various intervals.  Different intervals focused on inspecting different problem spaces.
	err = NewSpyglassEventIntervalRenderer("everything", BelongsInEverything).WriteRunData(storageDir, nil, customOrderedEvents, timeSuffix)
	if err != nil {
		errs = append(errs, err)
	}
	err = NewSpyglassEventIntervalRenderer("spyglass", BelongsInSpyglass).WriteRunData(storageDir, nil, customOrderedEvents, timeSuffix)
	if err != nil {
		errs = append(errs, err)
	}
	err = NewSpyglassEventIntervalRenderer("kube-apiserver", BelongsInKubeAPIServer).WriteRunData(storageDir, nil, customOrderedEvents, timeSuffix)
	if err != nil {
		errs = append(errs, err)
	}
	err = NewSpyglassEventIntervalRenderer("operators", BelongsInOperatorRollout).WriteRunData(storageDir, nil, customOrderedEvents, timeSuffix)
	if err != nil {
		errs = append(errs, err)
	}
	err = NewPodEventIntervalRenderer().WriteRunData(storageDir, nil, customOrderedEvents, timeSuffix)
	if err != nil {
		errs = append(errs, err)
	}
	err = NewIngressServicePodIntervalRenderer().WriteRunData(storageDir, nil, customOrderedEvents, timeSuffix)
	if err != nil {
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}

func (*timelineSerializer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
