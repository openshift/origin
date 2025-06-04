package sampler

import (
	"fmt"

	"github.com/openshift/origin/pkg/disruption/backend"
)

func NewResponseChecker() ResponseChecker {
	return &checker{}
}

type checker struct{}

func (c checker) CheckResponse(rr backend.RequestResponse) error {
	resp := rr.Response
	if rr.DNSErr != nil {
		return &KnownError{category: "DNSError", err: rr.DNSErr}
	}

	if rr.ResponseBodyReadErr != nil {
		// if we have failed to read the response body the
		// sample is deemed to have failed.
		return &KnownError{category: "NeedsTriage", err: fmt.Errorf("error while reading response body: %w", rr.ResponseBodyReadErr)}
	}

	if _, retry := backend.IsRetryAfter(resp); retry {
		if rr.ShutdownInProgress() {
			// We will deem a Retry-After response as a failure except while
			// the server is shutting down and is sending 429 to request(s)
			// that are arriving late. (this points to a faulty load balancer)
			return &KnownError{
				category: "FaultyLoadBalancer",
				err:      fmt.Errorf("very late request: %v body: %v", resp.Status, string(rr.ResponseBody)),
			}
		}
		// For now, any other retry-after is deemed as error since we don't
		// expect the server to be sending retry-after in CI.
		return &KnownError{
			category: "APIServerAvailability",
			err:      fmt.Errorf("server overwhelmed: %v body: %v", resp.Status, string(rr.ResponseBody)),
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode > 399 {
		return &KnownError{
			category: "APIServerAvailability",
			err:      fmt.Errorf("unexpected HTTP status code: %v body: %v", resp.Status, string(rr.ResponseBody)),
		}
	}
	return nil
}

func (c checker) CheckError(err error) error {
	// TODO: check for obvious error type and determine the category, to
	//  begin with, we could resolve the following errors:
	//    a) "no route to host" or "io timeout": these can be
	//        safely considered network error
	//    b) "connection refused": needs triage to begin with
	//    c) "connection reset":  needs triage to begin with
	return err
}
