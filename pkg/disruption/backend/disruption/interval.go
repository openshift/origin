package disruption

import (
	"github.com/openshift/origin/pkg/disruption/backend"
	backendsampler "github.com/openshift/origin/pkg/disruption/backend/sampler"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

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
func NewIntervalTracker(delegate backendsampler.SampleCollector, monitor backend.Monitor, eventRecorder events.EventRecorder,
	locator, name string, connType monitorapi.BackendConnectionType) *intervalTracker {
	handler := newCIHandler(monitor, eventRecorder, locator, name, connType)
	return &intervalTracker{
		delegate: delegate,
		handler:  handler,
	}
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

func (d *intervalTracker) Collect(bs backend.SampleResult) {
	// we receive sample in ordered sequence, 1, 2, ... n
	if d.delegate != nil {
		d.delegate.Collect(bs)
	}
	d.collect(bs)
}

func (d *intervalTracker) collect(bs backend.SampleResult) {
	if bs.Sample == nil {
		// no more sample arriving, do cleanup
		if d.previous.Sample != nil {
			d.handler.CloseInterval(d.previous)
		}
		return
	}

	current := bs
	if d.previous.Sample == nil {
		// this is the very first sample
		if !current.Succeeded() {
			d.handler.UnavailableStarted(current)
		}
		d.previous = current
		return
	}

	// 2nd or consecutive sample(s)
	previous := d.previous
	d.previous = current

	if previous.Succeeded() && current.Succeeded() {
		return
	}
	if !previous.Succeeded() && !current.Succeeded() {
		if previous.Error() == current.Error() {
			return
		}
	}

	d.handler.CloseInterval(current)
	if current.Succeeded() {
		d.handler.AvailableStarted(current)
		return
	}
	d.handler.UnavailableStarted(current)
}
