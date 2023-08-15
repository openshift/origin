package monitor

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

type recorder struct {
	lock   sync.Mutex
	events monitorapi.Intervals

	recordedResourceLock sync.Mutex
	recordedResources    monitorapi.ResourcesMap
}

// NewRecorder creates a recorder that can  be used to store events
func NewRecorder() monitorapi.Recorder {
	return &recorder{
		recordedResources: monitorapi.ResourcesMap{},
	}
}

var _ monitorapi.Recorder = &recorder{}

func (m *recorder) CurrentResourceState() monitorapi.ResourcesMap {
	m.recordedResourceLock.Lock()
	defer m.recordedResourceLock.Unlock()

	ret := monitorapi.ResourcesMap{}
	for resourceType, instanceResourceMap := range m.recordedResources {
		retInstance := monitorapi.InstanceMap{}
		for instanceKey, obj := range instanceResourceMap {
			retInstance[instanceKey] = obj.DeepCopyObject()
		}
		ret[resourceType] = retInstance
	}

	return ret
}

func (m *recorder) RecordResource(resourceType string, obj runtime.Object) {
	m.recordedResourceLock.Lock()
	defer m.recordedResourceLock.Unlock()

	recordedResource, ok := m.recordedResources[resourceType]
	if !ok {
		recordedResource = monitorapi.InstanceMap{}
		m.recordedResources[resourceType] = recordedResource
	}

	newMetadata, err := meta.Accessor(obj)
	if err != nil {
		// coding error
		panic(err)
	}
	key := monitorapi.InstanceKey{
		Namespace: newMetadata.GetNamespace(),
		Name:      newMetadata.GetName(),
		UID:       fmt.Sprintf("%v", newMetadata.GetUID()),
	}

	toStore := obj.DeepCopyObject()
	// without metadata, just stomp in the new value, we can't add annotations
	if newMetadata == nil {
		recordedResource[key] = toStore
		return
	}

	newAnnotations := newMetadata.GetAnnotations()
	if newAnnotations == nil {
		newAnnotations = map[string]string{}
	}
	existingResource, ok := recordedResource[key]
	if !ok {
		if newMetadata != nil {
			newAnnotations[monitorapi.ObservedUpdateCountAnnotation] = "1"
			newAnnotations[monitorapi.ObservedRecreationCountAnnotation] = "0"
			newMetadata.SetAnnotations(newAnnotations)
		}
		recordedResource[key] = toStore
		return
	}

	existingMetadata, _ := meta.Accessor(existingResource)
	// without metadata, just stomp in the new value, we can't add annotations
	if existingMetadata == nil {
		recordedResource[key] = toStore
		return
	}

	existingAnnotations := existingMetadata.GetAnnotations()
	if existingAnnotations == nil {
		existingAnnotations = map[string]string{}
	}
	existingUpdateCountStr := existingAnnotations[monitorapi.ObservedUpdateCountAnnotation]
	if existingUpdateCount, err := strconv.ParseInt(existingUpdateCountStr, 10, 32); err != nil {
		newAnnotations[monitorapi.ObservedUpdateCountAnnotation] = "1"
	} else {
		newAnnotations[monitorapi.ObservedUpdateCountAnnotation] = fmt.Sprintf("%d", existingUpdateCount+1)
	}

	// set the recreate count. increment if the UIDs don't match
	existingRecreateCountStr := existingAnnotations[monitorapi.ObservedUpdateCountAnnotation]
	if existingMetadata.GetUID() != newMetadata.GetUID() {
		if existingRecreateCount, err := strconv.ParseInt(existingRecreateCountStr, 10, 32); err != nil {
			newAnnotations[monitorapi.ObservedRecreationCountAnnotation] = existingRecreateCountStr
		} else {
			newAnnotations[monitorapi.ObservedRecreationCountAnnotation] = fmt.Sprintf("%d", existingRecreateCount+1)
		}
	} else {
		newAnnotations[monitorapi.ObservedRecreationCountAnnotation] = existingRecreateCountStr
	}

	newMetadata.SetAnnotations(newAnnotations)
	recordedResource[key] = toStore
	return
}

// Record captures one or more conditions at the current time. All conditions are recorded
// in monotonic order as EventInterval objects.
func (m *recorder) Record(conditions ...monitorapi.Condition) {
	m.RecordAt(time.Now().UTC(), conditions...)
}

// AddIntervals provides a mechanism to directly inject eventIntervals
func (m *recorder) AddIntervals(eventIntervals ...monitorapi.Interval) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.events = append(m.events, eventIntervals...)
}

// StartInterval inserts a record at time t with the provided condition and returns an opaque
// locator to the interval. The caller may close the sample at any point by invoking EndInterval().
func (m *recorder) StartInterval(t time.Time, condition monitorapi.Condition) int {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.events = append(m.events, monitorapi.Interval{
		Condition: condition,
		From:      t,
	})
	return len(m.events) - 1
}

// EndInterval updates the To of the interval started by StartInterval if it is greater than
// the from.
func (m *recorder) EndInterval(startedInterval int, t time.Time) *monitorapi.Interval {
	m.lock.Lock()
	defer m.lock.Unlock()
	if startedInterval < len(m.events) {
		if m.events[startedInterval].From.Before(t) {
			m.events[startedInterval].To = t
		}
		t := m.events[startedInterval]
		return &t
	}
	return nil
}

// RecordAt captures one or more conditions at the provided time. All conditions are recorded
// as EventInterval objects.
func (m *recorder) RecordAt(t time.Time, conditions ...monitorapi.Condition) {
	if len(conditions) == 0 {
		return
	}
	intervals := monitorapi.Intervals{}
	for _, condition := range conditions {
		intervals = append(intervals, monitorapi.Interval{
			Condition: condition,
			From:      t,
			To:        t,
		})
	}
	m.AddIntervals(intervals...)
}

func (m *recorder) snapshot() monitorapi.Intervals {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.events
}

// Intervals returns all events that occur between from and to, including
// any sampled conditions that were encountered during that period.
// Intervals are returned in order of their occurrence. The returned slice
// is a copy of the monitor's state and is safe to update.
func (m *recorder) Intervals(from, to time.Time) monitorapi.Intervals {
	events := m.snapshot()
	// we must sort *before*, we use the slice function
	sort.Sort(events)
	return events.Slice(from, to)
}
