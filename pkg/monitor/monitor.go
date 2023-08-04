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

// Monitor records events that have occurred in memory and can also periodically
// sample results.
type Monitor struct {
	lock           sync.Mutex
	events         monitorapi.Intervals
	unsortedEvents monitorapi.Intervals

	recordedResourceLock sync.Mutex
	recordedResources    monitorapi.ResourcesMap
}

// NewMonitor creates a monitor with the default sampling interval.
func NewMonitor() *Monitor {
	return &Monitor{
		recordedResources: monitorapi.ResourcesMap{},
	}
}

var _ Interface = &Monitor{}

func (m *Monitor) CurrentResourceState() monitorapi.ResourcesMap {
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

func (m *Monitor) RecordResource(resourceType string, obj runtime.Object) {
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
func (m *Monitor) Record(conditions ...monitorapi.Condition) {
	if len(conditions) == 0 {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	t := time.Now().UTC()
	for _, condition := range conditions {
		m.events = append(m.events, monitorapi.Interval{
			Condition: condition,
			From:      t,
			To:        t,
		})
	}
}

// AddIntervals provides a mechanism to directly inject eventIntervals
func (m *Monitor) AddIntervals(eventIntervals ...monitorapi.Interval) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.events = append(m.events, eventIntervals...)
}

// StartInterval inserts a record at time t with the provided condition and returns an opaque
// locator to the interval. The caller may close the sample at any point by invoking EndInterval().
func (m *Monitor) StartInterval(t time.Time, condition monitorapi.Condition) int {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.unsortedEvents = append(m.unsortedEvents, monitorapi.Interval{
		Condition: condition,
		From:      t,
	})
	return len(m.unsortedEvents) - 1
}

// EndInterval updates the To of the interval started by StartInterval if it is greater than
// the from.
func (m *Monitor) EndInterval(startedInterval int, t time.Time) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if startedInterval < len(m.unsortedEvents) {
		if m.unsortedEvents[startedInterval].From.Before(t) {
			m.unsortedEvents[startedInterval].To = t
		}
	}
}

// RecordAt captures one or more conditions at the provided time. All conditions are recorded
// as EventInterval objects.
func (m *Monitor) RecordAt(t time.Time, conditions ...monitorapi.Condition) {
	if len(conditions) == 0 {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, condition := range conditions {
		m.unsortedEvents = append(m.unsortedEvents, monitorapi.Interval{
			Condition: condition,
			From:      t,
			To:        t,
		})
	}
}

func (m *Monitor) snapshot() (monitorapi.Intervals, monitorapi.Intervals) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.events, m.unsortedEvents
}

// Intervals returns all events that occur between from and to, including
// any sampled conditions that were encountered during that period.
// Intervals are returned in order of their occurrence. The returned slice
// is a copy of the monitor's state and is safe to update.
func (m *Monitor) Intervals(from, to time.Time) monitorapi.Intervals {
	sortedEvents, unsortedEvents := m.snapshot()

	intervals := mergeIntervals(sortedEvents.Slice(from, to), unsortedEvents.CopyAndSort(from, to))

	return intervals
}

// mergeEvents returns a sorted list of all events provided as sources. This could be
// more efficient by requiring all sources to be sorted and then performing a zipper
// merge.
func mergeIntervals(sets ...monitorapi.Intervals) monitorapi.Intervals {
	total := 0
	for _, set := range sets {
		total += len(set)
	}
	merged := make(monitorapi.Intervals, 0, total)
	for _, set := range sets {
		merged = append(merged, set...)
	}
	sort.Sort(merged)
	return merged
}
