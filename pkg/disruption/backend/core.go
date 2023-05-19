package backend

import (
	"fmt"
	"net/http"
	"time"

	"github.com/openshift/origin/pkg/disruption/sampler"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
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
	if err != nil {
		err = fmt.Errorf("sample error: %v", s.Sample.Err)
	}
	if s.DNSErr != nil {
		err = fmt.Errorf("DNS error: %v - %v", s.DNSErr, err)
	}
	return err
}

// Monitor abstracts the subset of the Monitor API used by the disruption test.
type Monitor interface {
	StartInterval(t time.Time, condition monitorapi.Condition) int
	EndInterval(startedInterval int, t time.Time)
}
