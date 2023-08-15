package backend

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openshift/origin/pkg/disruption/sampler"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/client-go/tools/events"
)

// Client sends a given request to the server, and
// returns the response from the server.
type Client interface {
	Do(*http.Request) (*http.Response, error)
}

type ClientFunc func(*http.Request) (*http.Response, error)

func (f ClientFunc) Do(r *http.Request) (*http.Response, error) {
	return f(r)
}

// SampleResult holds the result of a request (sample) sent to the server
type SampleResult struct {
	RequestResponse
	Sample *sampler.Sample
}

func (s SampleResult) Succeeded() bool { return s.Sample.Err == nil }
func (s SampleResult) Error() string   { return s.Sample.Err.Error() }
func (s SampleResult) Err() error      { return s.Sample.Err }
func (s SampleResult) AggregateErr() error {
	err := s.Sample.Err
	if s.ShutdownResponseHeaderParseErr != nil {
		return fmt.Errorf("primary err: %v, shutdown response parse error: %v", err, s.ShutdownResponseHeaderParseErr)
	}
	return err
}

// WantEventRecorderAndMonitorRecorder allows the test driver to pass on
// the shared event recorder and the monitor instance.
type WantEventRecorderAndMonitorRecorder interface {
	SetEventRecorder(events.EventRecorder)
	SetMonitorRecorder(monitorRecorder monitorapi.RecorderWriter)
}

// HostNameDecoder is responsible for decoding the
// APIServerIdentity into the human readable hostname.
type HostNameDecoder interface {
	Decode(string) string
}

// HostNameDecoderWithRunner is a HostNameDecoder and also have a Run method
// that runs asynchronously in order to get the host name(s) from the cluster.
type HostNameDecoderWithRunner interface {
	HostNameDecoder
	Run(stop context.Context) (done context.Context)
}
