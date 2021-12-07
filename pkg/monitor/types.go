package monitor

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/runtime"
)

type IntervalCreationFunc func(intervals monitorapi.Intervals, beginning, end time.Time) monitorapi.Intervals

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

type sample struct {
	at         time.Time
	conditions []*monitorapi.Condition
}
