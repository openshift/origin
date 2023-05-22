package sampler

import (
	"fmt"

	"github.com/openshift/origin/pkg/disruption/backend"
)

// DefaultResponseChecker is the default ResponseChecker implementation,
// it checks the result and determines whether the request
// has failed or succeeded.
func DefaultResponseChecker(rr backend.RequestResponse) error {
	resp := rr.Response
	if _, retry := backend.IsRetryAfter(resp); retry && rr.ShutdownInProgress() {
		// We will deem a Retry-After response as a failure except while
		// the server is shutting down and is sending 429 to request(s)
		// that are arriving late.
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode > 399 {
		return fmt.Errorf("unexpected HTTP status code: %v body: %v", resp.Status, string(rr.ResponseBody))
	}
	return nil
}
