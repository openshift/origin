package intervalserializer

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type intervalSerializer struct {
}

func NewIntervalSerializer() monitortestframework.MonitorTest {
	return &intervalSerializer{}
}

func (w *intervalSerializer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *intervalSerializer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *intervalSerializer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*intervalSerializer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*intervalSerializer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*intervalSerializer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return monitorserialization.EventsToFile(filepath.Join(storageDir, fmt.Sprintf("e2e-events%s.json", timeSuffix)), finalIntervals)
}

func (*intervalSerializer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
