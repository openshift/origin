package monitor

import (
	"context"
	"time"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/runtime"
)

type SamplerFunc func(time.Time) []*monitorapi.Condition

type Interface interface {
	Intervals(from, to time.Time) monitorapi.Intervals
	Conditions(from, to time.Time) monitorapi.Intervals
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

	AddSampler(fn SamplerFunc)
}

type ConditionalSampler interface {
	ConditionWhenFailing(context.Context, *monitorapi.Condition) SamplerFunc
	WhenFailing(context.Context, *monitorapi.Condition)
}

// SampleFunc takes a bool representing "were you failing last time" and returns
//  1. a nil condition if there is no noteworthy state change
//  2. a condition if there is a noteworthy state change
//  3. a bool indicating if it is currently available
type SampleFunc func(previouslyAvailable bool) (edgeCondition *monitorapi.Condition, currentlyAvailable bool)

type sample struct {
	at         time.Time
	conditions []*monitorapi.Condition
}

// StartEventIntervalRecorder is non-blocking and must stop on a context cancel.  It is expected to call the recorder
// an is encouraged to use EventIntervals to record edges.  They are often paired with TestSuite.SyntheticEventTests.
type StartEventIntervalRecorderFunc func(ctx context.Context, recorder Recorder, clusterConfig *rest.Config) error
