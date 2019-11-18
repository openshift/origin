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

package migrators

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/utils/pointer"
)

// isConnectionRefusedError checks if the error string include "connection refused"
// TODO: find a "go-way" to detect this error, probably using *os.SyscallError
func isConnectionRefusedError(err error) bool {
	return strings.Contains(err.Error(), "connection refused")
}

// canRetry returns false if the provided error indicates a retry is
// impossible. It returns true if the error is possibly temporary. It returns
// nil for all other error where it is unclear.
func canRetry(err error) *bool {
	switch {
	case err == nil:
		return nil
	case errors.IsNotFound(err), errors.IsMethodNotSupported(err):
		return pointer.BoolPtr(false)
	case errors.IsConflict(err), errors.IsServerTimeout(err), errors.IsTooManyRequests(err), net.IsProbableEOF(err), net.IsConnectionReset(err), net.IsNoRoutesError(err), isConnectionRefusedError(err):
		return pointer.BoolPtr(true)
	default:
		return nil
	}
}
