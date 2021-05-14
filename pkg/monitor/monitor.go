package monitor

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// Monitor records events that have occurred in memory and can also periodically
// sample results.
type Monitor struct {
	interval            time.Duration
	samplers            []SamplerFunc
	intervalCreationFns []IntervalCreationFunc

	lock           sync.Mutex
	events         monitorapi.Intervals
	unsortedEvents monitorapi.Intervals
	samples        []*sample
}

// NewMonitor creates a monitor with the default sampling interval.
func NewMonitor() *Monitor {
	return NewMonitorWithInterval(15 * time.Second)
}

// NewMonitorWithInterval creates a monitor that samples at the provided
// interval.
func NewMonitorWithInterval(interval time.Duration) *Monitor {
	return &Monitor{
		interval: interval,
	}
}

var _ Interface = &Monitor{}

// StartSampling starts sampling every interval until the provided context is done.
// A sample is captured when the context is closed.
func (m *Monitor) StartSampling(ctx context.Context) {
	if m.interval == 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		hasConditions := false
		for {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				hasConditions = m.sample(hasConditions)
				return
			}
			hasConditions = m.sample(hasConditions)
		}
	}()
}

// AddSampler adds a sampler function to the list of samplers to run every interval.
// Conditions discovered this way are recorded with a start and end time if they persist
// across multiple sampling intervals.
func (m *Monitor) AddSampler(fn SamplerFunc) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.samplers = append(m.samplers, fn)
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
		m.events = append(m.events, monitorapi.EventInterval{
			Condition: condition,
			From:      t,
			To:        t,
		})
	}
}

// StartInterval inserts a record at time t with the provided condition and returns an opaque
// locator to the interval. The caller may close the sample at any point by invoking EndInterval().
func (m *Monitor) StartInterval(t time.Time, condition monitorapi.Condition) int {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.unsortedEvents = append(m.unsortedEvents, monitorapi.EventInterval{
		Condition: condition,
		From:      t,
	})
	return len(m.unsortedEvents) - 1
}

// EndInterval updates the To of the interval started by StartInterval if t is greater than
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
		m.unsortedEvents = append(m.unsortedEvents, monitorapi.EventInterval{
			Condition: condition,
			From:      t,
			To:        t,
		})
	}
}

func (m *Monitor) sample(hasPrevious bool) bool {
	m.lock.Lock()
	samplers := m.samplers
	m.lock.Unlock()

	now := time.Now().UTC()
	var conditions []*monitorapi.Condition
	for _, fn := range samplers {
		conditions = append(conditions, fn(now)...)
	}
	if len(conditions) == 0 {
		if !hasPrevious {
			return false
		}
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	m.samples = append(m.samples, &sample{
		at:         now,
		conditions: conditions,
	})
	return len(conditions) > 0
}

func (m *Monitor) snapshot() ([]*sample, monitorapi.Intervals, monitorapi.Intervals) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.samples, m.events, m.unsortedEvents
}

// Conditions returns all conditions that were sampled in the interval
// between from and to. If that does not include a sample interval, no
// results will be returned. Intervals are returned in order of
// their first sampling. A condition that was only sampled once is
// returned with from == to. No duplicate conditions are returned
// unless a sampling interval did not report that value.
func (m *Monitor) Conditions(from, to time.Time) monitorapi.Intervals {
	samples, _, _ := m.snapshot()
	return filterSamples(samples, from, to)
}

// EventIntervals returns all events that occur between from and to, including
// any sampled conditions that were encountered during that period.
// Intervals are returned in order of their occurrence. The returned slice
// is a copy of the monitor's state and is safe to update.
func (m *Monitor) Intervals(from, to time.Time) monitorapi.Intervals {
	samples, sortedEvents, unsortedEvents := m.snapshot()

	intervals := mergeIntervals(sortedEvents.Slice(from, to), unsortedEvents.CopyAndSort(from, to), filterSamples(samples, from, to))
	originalLen := len(intervals)

	// create additional intervals from events
	for _, createIntervals := range m.intervalCreationFns {
		intervals = append(intervals, createIntervals(intervals, from, to)...)
	}

	// we must sort the result
	if len(intervals) != originalLen {
		sort.Sort(intervals)
	}

	return intervals
}

// filterSamples converts the sorted samples that are within [from,to) to a set of
// intervals.
// TODO: simplify this by having the monitor samplers produce intervals themselves
//   and make the streaming print logic simply show transitions.
func filterSamples(samples []*sample, from, to time.Time) monitorapi.Intervals {
	if len(samples) == 0 {
		return nil
	}

	if !from.IsZero() {
		first := sort.Search(len(samples), func(i int) bool {
			return samples[i].at.After(from)
		})
		if first == -1 {
			return nil
		}
		samples = samples[first:]
	}

	if !to.IsZero() {
		for i, sample := range samples {
			if sample.at.After(to) {
				samples = samples[:i]
				break
			}
		}
	}
	if len(samples) == 0 {
		return nil
	}

	intervals := make(monitorapi.Intervals, 0, len(samples)*2)
	last, next := make(map[monitorapi.Condition]*monitorapi.EventInterval), make(map[monitorapi.Condition]*monitorapi.EventInterval)
	for _, sample := range samples {
		for _, condition := range sample.conditions {
			interval, ok := last[*condition]
			if ok {
				interval.To = sample.at
				next[*condition] = interval
				continue
			}
			intervals = append(intervals, monitorapi.EventInterval{
				Condition: *condition,
				From:      sample.at,
				To:        sample.at.Add(time.Second),
			})
			next[*condition] = &intervals[len(intervals)-1]
		}
		for k := range last {
			delete(last, k)
		}
		last, next = next, last
	}
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
