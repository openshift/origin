package monitor

import (
	"context"
	"time"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/runtime"
)

type Interface interface {
	Intervals(from, to time.Time) monitorapi.Intervals
	CurrentResourceState() monitorapi.ResourcesMap
}

type Recorder interface {
	// RecordResource stores a resource for later serialization.  Deletion is not tracked, so this can be used
	// to determine the final state of resource that are deleted in a namespace.
	// Annotations are added to indicate number of updates and the number of recreates.
	RecordResource(resourceType string, obj runtime.Object)

	Record(conditions ...monitorapi.Condition)
	RecordAt(t time.Time, conditions ...monitorapi.Condition)

	StartInterval(t time.Time, condition monitorapi.Condition) int
	EndInterval(startedInterval int, t time.Time)
}

// StartEventIntervalRecorder is non-blocking and must stop on a context cancel.  It is expected to call the recorder
// an is encouraged to use EventIntervals to record edges.  They are often paired with TestSuite.SyntheticEventTests.
type StartEventIntervalRecorderFunc func(ctx context.Context, recorder Recorder, clusterConfig *rest.Config, lb backend.LoadBalancerType) error
