package trackedresources_serializer

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type trackedResourcesSerializer struct {
}

func NewTrackedResourcesSerializer() invariants.InvariantTest {
	return &trackedResourcesSerializer{}
}

func (w *trackedResourcesSerializer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *trackedResourcesSerializer) CollectData(ctx context.Context, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	// because we are sharing a recorder that we're streaming into, we don't need to have a separate data collection step.
	return nil, nil, nil
}

func (*trackedResourcesSerializer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*trackedResourcesSerializer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*trackedResourcesSerializer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	errors := []error{}

	// write out the current state of resources that we explicitly tracked.
	for resourceType, instanceMap := range finalResourceState {
		targetFile := fmt.Sprintf("resource-%s%s.zip", resourceType, timeSuffix)
		if err := monitorserialization.InstanceMapToFile(filepath.Join(storageDir, targetFile), resourceType, instanceMap); err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}

func (*trackedResourcesSerializer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
