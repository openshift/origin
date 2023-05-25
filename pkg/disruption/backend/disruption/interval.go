package disruption

import (
	"github.com/openshift/origin/pkg/disruption/backend"
	backendsampler "github.com/openshift/origin/pkg/disruption/backend/sampler"
	"k8s.io/client-go/tools/events"
)

// NewIntervalTracker returns a SampleCollector that does the following:
//
//   - goes through each sample result, and determines the disruption
//     interval(s) - unavailable, and available again, and then
//
//   - records each interval window in CI appropriately
//
//     delegate: the next SampleCollector in the chain to be invoked
//     monitor: Monitor API to start and end an interval in CI
//     eventRecorder: to create events associated with the intervals
//     locator: the CI locator assigned to this disruption test
//     name: name of the disruption test
//     connType: user specified BackendConnectionType used in this test
//
// For example given the following sequence of samples
//
//	s1:success s2:err1 s3:err1 s4:err1 s5:success s6:success s7:err1 s8:success
//
// it will generate the following disruption intervals
//
//	unavailable[s2,s4] available[s5,s6] unavailable[s7] available[s8]
func NewIntervalTracker(delegate backendsampler.SampleCollector, descriptor backend.TestDescriptor, monitor backend.Monitor,
	eventRecorder events.EventRecorder) (backendsampler.SampleCollector, backend.WantEventRecorderAndMonitor) {
	handler := newCIHandler(descriptor, monitor, eventRecorder)
	return &intervalTracker{
		delegate: delegate,
		handler:  handler,
	}, handler
}

// intervalHandler is an internal interface that receives the calculated
// disruption interval(s) and handle them. The caller must ensure that
// no existing interval is left open while creating a new one.
// The expected sequence is:
//   - UnavailableStarted, CloseInterval, AvailableStarted, CloseInterval
//   - UnavailableStarted, CloseInterval, UnavailableStarted, CloseInterval
//
// NOTE: This is intentionally not exported, it helps in writing unit tests
type intervalHandler interface {
	// UnavailableStarted is called when a disruption interval starts
	// with the given sample that has failed.
	UnavailableStarted(backend.SampleResult)

	// AvailableStarted is called when a disruption interval ends
	// with the given sample that has succeeded.
	AvailableStarted(backend.SampleResult)

	// CloseInterval should close the current open interval
	CloseInterval(backend.SampleResult)
}

type intervalTracker struct {
	delegate backendsampler.SampleCollector
	handler  intervalHandler

	previous backend.SampleResult
}

func (t *intervalTracker) Collect(bs backend.SampleResult) {
	// we receive sample in ordered sequence, 1, 2, ... n
	if t.delegate != nil {
		t.delegate.Collect(bs)
	}
	t.collect(bs)
}

func (t *intervalTracker) collect(bs backend.SampleResult) {
	if bs.Sample == nil {
		// no more sample arriving, do cleanup
		if t.previous.Sample != nil {
			t.handler.CloseInterval(t.previous)
		}
		return
	}

	current := bs
	if t.previous.Sample == nil {
		// this is the very first sample
		if !current.Succeeded() {
			t.handler.UnavailableStarted(current)
		}
		t.previous = current
		return
	}

	// 2nd or consecutive sample(s)
	previous := t.previous
	t.previous = current

	if previous.Succeeded() && current.Succeeded() {
		return
	}
	if !previous.Succeeded() && !current.Succeeded() {
		if previous.Error() == current.Error() {
			return
		}
	}

	t.handler.CloseInterval(current)
	if current.Succeeded() {
		t.handler.AvailableStarted(current)
		return
	}
	t.handler.UnavailableStarted(current)
}
