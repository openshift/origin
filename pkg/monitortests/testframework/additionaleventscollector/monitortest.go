package additionaleventscollector

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type additionalEventsCollector struct {
}

func NewIntervalSerializer() monitortestframework.MonitorTest {
	return &additionalEventsCollector{}
}

func (w *additionalEventsCollector) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *additionalEventsCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *additionalEventsCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	var err error
	additionIntervals := monitorapi.Intervals{}

	// read events from other test processes (individual tests for instance) that happened during this run.
	// this happens during upgrade tests to pass information back to the main monitor.
	if len(storageDir) > 0 {
		var intervalsFromStorage monitorapi.Intervals
		err = filepath.WalkDir(storageDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			// upstream framework starting in 1.26 fixed how the AfterEach is
			// invoked, now it's always invoked and that results in more files
			// than we anticipated before, see here:
			// https://github.com/kubernetes/kubernetes/blob/v1.26.0/test/e2e/framework/framework.go#L382
			// our files have double underscore whereas upstream has only single
			// so for now we'll skip everything else for summaries
			//
			// TODO: TRT will need to double check this longterm what they want
			// to do with these extra files
			if !strings.HasPrefix(d.Name(), "AdditionalEvents__") {
				return nil
			}
			saved, _ := monitorserialization.EventsFromFile(path)
			intervalsFromStorage = append(intervalsFromStorage, saved...)
			return nil
		})

		if len(intervalsFromStorage) > 0 {
			additionIntervals = append(additionIntervals, intervalsFromStorage.Cut(time.Time{}, time.Time{})...)
		}
	}

	return additionIntervals, nil, err
}

func (*additionalEventsCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*additionalEventsCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*additionalEventsCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*additionalEventsCollector) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
