package sampler

import (
	"context"
	"net/http"

	"github.com/openshift/origin/pkg/disruption/backend"
)

// Requestor knows how to construct a new request to the target backend
type Requestor interface {
	// GetBaseURL returns the base URL of the request that is sent to the server.
	// NOTE: we need this to support the the existing BackendSampler.
	GetBaseURL() string

	// NewHTTPRequest returns a new HTTP request, it can use the given
	// sample ID to generate a new request.
	NewHTTPRequest(ctx context.Context, sampleID uint64) (*http.Request, error)
}

// ResponseChecker checks the given HTTP Response object and optionally the
// response body that has been successfully read.
// If it returns an error, the sample is deemed to have failed.
// The given http.Response object must not be nil
// The response body maybe empty depending on the kind of
// request sent to the server.
type ResponseChecker interface {
	// CheckError checks the the given error for any known types
	CheckError(error) error

	// CheckResponse checks the given HTTP Response object and
	// optionally the response body that has been successfully read.
	// If it returns an error, the sample is deemed to have failed.
	// The given http.Response object must not be nil
	// The response body maybe empty depending on the kind of
	// request sent to the server.
	CheckResponse(backend.RequestResponse) error
}

type ResponseCheckerFunc func(backend.RequestResponse) error

func (c ResponseCheckerFunc) CheckResponse(rr backend.RequestResponse) error {
	return c(rr)
}

// SampleCollector collects one sample result at a time in the following
// sequence: 1, 2, 3 ... N.
// When there are no more samples, an empty SampleResult is provided
// as a marker to indicate no more results are arriving.
type SampleCollector interface {
	Collect(backend.SampleResult)
}
