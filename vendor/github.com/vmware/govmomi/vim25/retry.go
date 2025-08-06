// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package vim25

import (
	"context"
	"time"

	"github.com/vmware/govmomi/vim25/soap"
)

type RetryFunc func(err error) (retry bool, delay time.Duration)

// TemporaryNetworkError is deprecated. Use Retry() with RetryTemporaryNetworkError and retryAttempts instead.
func TemporaryNetworkError(n int) RetryFunc {
	return func(err error) (bool, time.Duration) {
		if IsTemporaryNetworkError(err) {
			// Don't retry if we're out of tries.
			if n--; n <= 0 {
				return false, 0
			}
			return true, 0
		}
		return false, 0
	}
}

// RetryTemporaryNetworkError returns a RetryFunc that returns IsTemporaryNetworkError(err)
func RetryTemporaryNetworkError(err error) (bool, time.Duration) {
	return IsTemporaryNetworkError(err), 0
}

// IsTemporaryNetworkError returns false unless the error implements
// a Temporary() bool method such as url.Error and net.Error.
// Otherwise, returns the value of the Temporary() method.
func IsTemporaryNetworkError(err error) bool {
	t, ok := err.(interface {
		// Temporary is implemented by url.Error and net.Error
		Temporary() bool
	})

	if !ok {
		// Not a Temporary error.
		return false
	}

	return t.Temporary()
}

type retry struct {
	roundTripper soap.RoundTripper

	// fn is a custom function that is called when an error occurs.
	// It returns whether or not to retry, and if so, how long to
	// delay before retrying.
	fn               RetryFunc
	maxRetryAttempts int
}

// Retry wraps the specified soap.RoundTripper and invokes the
// specified RetryFunc. The RetryFunc returns whether or not to
// retry the call, and if so, how long to wait before retrying. If
// the result of this function is to not retry, the original error
// is returned from the RoundTrip function.
// The soap.RoundTripper will return the original error if retryAttempts is specified and reached.
func Retry(roundTripper soap.RoundTripper, fn RetryFunc, retryAttempts ...int) soap.RoundTripper {
	r := &retry{
		roundTripper:     roundTripper,
		fn:               fn,
		maxRetryAttempts: 1,
	}

	if len(retryAttempts) == 1 {
		r.maxRetryAttempts = retryAttempts[0]
	}

	return r
}

func (r *retry) RoundTrip(ctx context.Context, req, res soap.HasFault) error {
	var err error

	for attempt := 0; attempt < r.maxRetryAttempts; attempt++ {
		err = r.roundTripper.RoundTrip(ctx, req, res)
		if err == nil {
			break
		}

		// Invoke retry function to see if another attempt should be made.
		if retry, delay := r.fn(err); retry {
			time.Sleep(delay)
			continue
		}

		break
	}

	return err
}
