package backend

import (
	"net/http"

	"github.com/openshift/origin/pkg/disruption/sampler"
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
