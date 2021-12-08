package backenddisruption

import (
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// TODO remove all this once the monitor package no longer bakes in direct knowledge of the backenddisruption package
type simpleMonitor struct {
	lock                   sync.Mutex
	unsortedEventIntervals monitorapi.Intervals
}

func newSimpleMonitor() *simpleMonitor {
	return &simpleMonitor{}
}

// StartInterval inserts a record at time t with the provided condition and returns an opaque
// locator to the interval. The caller may close the sample at any point by invoking EndInterval().
func (m *simpleMonitor) StartInterval(t time.Time, condition monitorapi.Condition) int {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.unsortedEventIntervals = append(m.unsortedEventIntervals, monitorapi.EventInterval{
		Condition: condition,
		From:      t,
	})
	return len(m.unsortedEventIntervals) - 1
}

// EndInterval updates the To of the interval started by StartInterval if t is greater than
// the from.
func (m *simpleMonitor) EndInterval(startedInterval int, t time.Time) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if startedInterval < len(m.unsortedEventIntervals) {
		if m.unsortedEventIntervals[startedInterval].From.Before(t) {
			m.unsortedEventIntervals[startedInterval].To = t
		}
	}
}

// Intervals returns all events that occur between from and to, including
// any sampled conditions that were encountered during that period.
// Intervals are returned in order of their occurrence. The returned slice
// is a copy of the monitor's state and is safe to update.
func (m *simpleMonitor) Intervals(from, to time.Time) monitorapi.Intervals {
	m.lock.Lock()
	defer m.lock.Unlock()

	return m.unsortedEventIntervals.CopyAndSort(from, to)
}
