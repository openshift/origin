/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package encryption

import (
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
)

// isConnectionRefusedError checks if the error string include "connection refused"
// TODO: find a "go-way" to detect this error, probably using *os.SyscallError
func isConnectionRefusedError(err error) bool {
	return strings.Contains(err.Error(), "connection refused")
}

// transientAPIError returns true if the provided error indicates that a retry
// against an HA server has a good chance to succeed.
func transientAPIError(err error) bool {
	switch {
	case err == nil:
		return false
	case errors.IsServerTimeout(err), errors.IsTooManyRequests(err), net.IsProbableEOF(err), net.IsConnectionReset(err), net.IsNoRoutesError(err), isConnectionRefusedError(err):
		return true
	default:
		return false
	}
}

func orError(a, b func(error) bool) func(error) bool {
	return func(err error) bool {
		return a(err) || b(err)
	}
}

func onErrorWithTimeout(timeout time.Duration, backoff wait.Backoff, errorFunc func(error) bool, fn func() error) error {
	var lastMatchingError error
	stopCh := time.After(timeout)
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		select {
		case <-stopCh:
			return false, wait.ErrWaitTimeout
		default:
		}
		err := fn()
		switch {
		case err == nil:
			return true, nil
		case errorFunc(err):
			lastMatchingError = err
			return false, nil
		default:
			return false, err
		}
	})
	if err == wait.ErrWaitTimeout && lastMatchingError != nil {
		err = lastMatchingError
	}
	return err
}
