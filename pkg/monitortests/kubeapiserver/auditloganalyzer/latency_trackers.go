package auditloganalyzer

import (
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	etcdLatencyTrackerKey = "apiserver.latency.k8s.io/etcd"
)

type eventWithLatencyTracker struct {
	Event   *auditv1.Event
	Latency time.Duration
}

type latencyMetricsForEventTrackers struct {
	Events []*eventWithLatencyTracker

	Median time.Duration

	FirstEventTimeStamp metav1.MicroTime
	LastEventTimeStamp  metav1.MicroTime
}

type slidingWindowCollectorForLatencyTracker struct {
	events []*eventWithLatencyTracker

	windowDuration time.Duration
	windowStart    time.Time
	windowEnd      time.Time

	latencyTracker string
}

func newEtcdLatencyTrackerAnalyser() *latencyTrackerAnalyser {
	collector := newSlidingWindowCollectorForLatencyTracker(30*time.Second, etcdLatencyTrackerKey)
	detector := newMedianDetectorForLatencyMetricsForEventTracker(10 * time.Second)
	analyser := newLatencyTrackerAnalyser(collector, detector)

	return analyser
}

func newSlidingWindowCollectorForLatencyTracker(windowDuration time.Duration, latencyTracker string) *slidingWindowCollectorForLatencyTracker {
	return &slidingWindowCollectorForLatencyTracker{
		windowDuration: windowDuration,
		latencyTracker: latencyTracker,
	}
}

func (it *slidingWindowCollectorForLatencyTracker) Collect(rawEvent *auditv1.Event) []*eventWithLatencyTracker {
	event, err := toEventWithLatencyTrackerFor(rawEvent, it.latencyTracker)
	if err != nil {
		framework.Logf("Failed extracting latency duration for an audit with ID: %v, for latencyTracker: %v, due to: %v", rawEvent.AuditID, it.latencyTracker, err)
		return nil
	}
	if event == nil {
		// the event doesn't contain the desired latency tracker
		return nil
	}

	if len(it.events) == 0 {
		// first event, init window
		it.windowStart = event.Event.RequestReceivedTimestamp.Time
		it.windowEnd = it.windowStart.Add(it.windowDuration)
		it.events = append(it.events, event)
		return nil
	}

	if event.Event.RequestReceivedTimestamp.Time.After(it.windowEnd) {
		// event is outside the current window, return current events and start a new window
		collectedEvents := it.events
		it.events = []*eventWithLatencyTracker{event}
		it.windowStart = event.Event.RequestReceivedTimestamp.Time
		it.windowEnd = it.windowStart.Add(it.windowDuration)
		return collectedEvents
	}

	// event is within the current window
	it.events = append(it.events, event)
	return nil
}

type medianDetectorForLatencyMetricsForEventTrackers struct {
	medianThreshold time.Duration
}

func newMedianDetectorForLatencyMetricsForEventTracker(medianThreshold time.Duration) *medianDetectorForLatencyMetricsForEventTrackers {
	return &medianDetectorForLatencyMetricsForEventTrackers{medianThreshold: medianThreshold}
}

func (d *medianDetectorForLatencyMetricsForEventTrackers) Detect(metrics *latencyMetricsForEventTrackers) bool {
	if metrics == nil {
		return false
	}
	return metrics.Median > d.medianThreshold
}

type latencyTrackerAnalyser struct {
	collector *slidingWindowCollectorForLatencyTracker
	detector  *medianDetectorForLatencyMetricsForEventTrackers
}

func newLatencyTrackerAnalyser(collector *slidingWindowCollectorForLatencyTracker, detector *medianDetectorForLatencyMetricsForEventTrackers) *latencyTrackerAnalyser {
	return &latencyTrackerAnalyser{collector: collector, detector: detector}
}

func (l *latencyTrackerAnalyser) Analyse(event *auditv1.Event) monitorapi.Intervals {
	collectedEvents := l.collector.Collect(event)
	if collectedEvents == nil {
		return monitorapi.Intervals{}
	}
	latencyMetricsForCollectedEvents := calculateLatencyMetricsForEventTrackers(collectedEvents)
	if l.detector.Detect(latencyMetricsForCollectedEvents) {
		// TODO: calculate the interval
	}
	return monitorapi.Intervals{}
}

func calculateLatencyMetricsForEventTrackers(events []*eventWithLatencyTracker) *latencyMetricsForEventTrackers {
	if len(events) == 0 {
		return &latencyMetricsForEventTrackers{}
	}

	sortedLatencies := eventsWithLatencyTrackerToDuration(events)

	return &latencyMetricsForEventTrackers{
		Events:              events,
		Median:              median(sortedLatencies),
		FirstEventTimeStamp: events[0].Event.RequestReceivedTimestamp,
		LastEventTimeStamp:  events[len(events)-1].Event.RequestReceivedTimestamp,
	}
}

func eventsWithLatencyTrackerToDuration(events []*eventWithLatencyTracker) []time.Duration {
	latencies := make([]time.Duration, len(events))
	for i, event := range events {
		latencies[i] = event.Latency
	}
	return latencies
}

func toEventWithLatencyTrackerFor(event *auditv1.Event, latencyTracker string) (*eventWithLatencyTracker, error) {
	for actualLatencyTracker, actualLatencyValue := range event.Annotations {
		if actualLatencyTracker != latencyTracker {
			continue
		}
		latencyDuration, err := time.ParseDuration(actualLatencyValue)
		if err != nil {
			return nil, fmt.Errorf(fmt.Sprintf("Error parsing %q=%v duration, err=%v", latencyTracker, actualLatencyValue, err))
		}
		return &eventWithLatencyTracker{Event: event, Latency: latencyDuration}, nil
	}
	return nil, nil
}

func mean(latency1, latency2 time.Duration) time.Duration {
	latency1Ns := latency1.Nanoseconds()
	latency2Ns := latency2.Nanoseconds()
	meanLatencyNs := (latency1Ns + latency2Ns) / 2
	return time.Duration(meanLatencyNs)
}

func median(latencies []time.Duration) time.Duration {
	var median time.Duration
	if len(latencies)%2 == 0 {
		latencies = latencies[len(latencies)/2-1 : len(latencies)/2+1]
		median = mean(latencies[0], latencies[1])
	} else {
		median = latencies[len(latencies)/2]
	}
	return median
}
