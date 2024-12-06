package disruption

import (
	"fmt"
	"strings"

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
func NewIntervalTracker(delegate backendsampler.SampleCollector, descriptor backend.TestDescriptor, monitorRecorder monitorapi.RecorderWriter,
	eventRecorder events.EventRecorder) (backendsampler.SampleCollector, backend.WantEventRecorderAndMonitorRecorder) {
	handler := newCIHandler(descriptor, monitorRecorder, eventRecorder)
	return &intervalTracker{
		delegate: delegate,
		handler:  handler,
	}, handler
}

// intervalHandler is an internal interface that receives the calculated
// disruption interval(s) and handle them.
// NOTE: This is intentionally not exported, it helps in writing unit tests
type intervalHandler interface {
	// Available is called when a disruption interval ends and we see
	// a series of successful samples in this range [from ... to).
	//  a) either from or to must not be nil
	//  b) for a window with a single sample, from and to can refer
	//     to the same sample in question.
	Available(from, to *backend.SampleResult)

	// Unavailable is called for a disruption interval when we see
	// a series of failed samples in this range [from ... to).
	//  a) either from or to must not be nil
	//  b) for a window with a single sample, from and to can refer
	//     to the same sample in question.
	Unavailable(from, to *backend.SampleResult)
}

type intervalTracker struct {
	delegate backendsampler.SampleCollector
	handler  intervalHandler

	from     *backend.SampleResult
	previous *backend.SampleResult
}

func (t *intervalTracker) Collect(bs backend.SampleResult) {
	// we receive sample in ordered sequence, 1, 2, ... n
	if t.delegate != nil {
		t.delegate.Collect(bs)
	}
	t.collect(bs)
}

func (t *intervalTracker) collect(result backend.SampleResult) {
	if result.Sample == nil {
		if from := t.from; from != nil {
			switch {
			case from.Succeeded():
				t.handler.Available(from, t.previous)
			default:
				t.handler.Unavailable(t.from, t.previous)
			}
		}
		// no more sample arriving, do cleanup
		return
	}

	current := &result
	if t.previous == nil {
		// this is the very first sample
		switch {
		case current.Succeeded():
			// this will ensure we have a "zero"
			t.handler.Available(current, current)
		default:
			// the very first sample failed, we will need to start
			// an Unavailable window from this sample.
			t.from = current
		}
		t.previous = current
		return
	}

	// 2nd or consecutive sample(s)
	previous := t.previous
	t.previous = current
	switch {
	case previous.Succeeded() && current.Succeeded():
		return
	case !previous.Succeeded() && !current.Succeeded():
		previousErrorCensored := previous.Error()
		currentErrorCensored := current.Error()
		// Censor sample-id from error message
		if previous.Sample != nil && current.Sample != nil {
			previousErrorCensored = strings.ReplaceAll(previous.Error(), fmt.Sprintf("&sample-id=%d", previous.Sample.ID), "")
			currentErrorCensored = strings.ReplaceAll(current.Error(), fmt.Sprintf("&sample-id=%d", current.Sample.ID), "")
		}

		if previousErrorCensored == currentErrorCensored {
			return
		}
		//  both previous and current failed, but with different errors
		t.handler.Unavailable(t.from, current)

	// if we are here, we have a transition
	//  a) previous sample failed, current is a success
	//  b) previous sample succeeded, current has failed
	case current.Succeeded():
		if t.from != nil {
			t.handler.Unavailable(t.from, current)
		}
	default:
		if t.from != nil {
			t.handler.Available(t.from, current)
		}
	}
	t.from = current
}
