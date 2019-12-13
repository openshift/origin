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
)

// ErrRetriable is a wrapper for an error that a migrator may use to indicate the
// specific error can be retried.
type ErrRetriable struct {
	error
}

func (ErrRetriable) Temporary() bool { return true }

// ErrNotRetriable is a wrapper for an error that a migrator may use to indicate the
// specific error cannot be retried.
type ErrNotRetriable struct {
	error
}

func (ErrNotRetriable) Temporary() bool { return false }

// TemporaryError is a wrapper interface that is used to determine if an error can be retried.
type TemporaryError interface {
	error
	// Temporary should return true if this is a temporary error
	Temporary() bool
}

// isConnectionRefusedError checks if the error string include "connection refused"
// TODO: find a "go-way" to detect this error, probably using *os.SyscallError
func isConnectionRefusedError(err error) bool {
	return strings.Contains(err.Error(), "connection refused")
}

// interpret adds retry information to the provided error. And it might change
// the error to nil.
func interpret(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.IsNotFound(err):
		// if the object is deleted, there is no need to migrate
		return nil
	case errors.IsMethodNotSupported(err):
		return ErrNotRetriable{err}
	case errors.IsConflict(err):
		return ErrRetriable{err}
	case errors.IsServerTimeout(err):
		return ErrRetriable{err}
	case errors.IsTooManyRequests(err):
		return ErrRetriable{err}
	case net.IsProbableEOF(err):
		return ErrRetriable{err}
	case net.IsConnectionReset(err):
		return ErrRetriable{err}
	case net.IsNoRoutesError(err):
		return ErrRetriable{err}
	case isConnectionRefusedError(err):
		return ErrRetriable{err}
	default:
		return err
	}
}

// canRetry returns false if the provided error indicates a retry is
// impossible. Otherwise it returns true.
func canRetry(err error) bool {
	err = interpret(err)
	if temp, ok := err.(TemporaryError); ok && !temp.Temporary() {
		return false
	}
	return true
}
