package retryableerror

import (
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// Error represents an error for an operation that should be retried after the
// specified duration.
type Error interface {
	error
	// After is the time period after which the operation that caused the
	// error should be retried.
	After() time.Duration
}

// New returns a new RetryableError with the given error and time period.
func New(err error, after time.Duration) Error {
	return retryableError{err, after}
}

type retryableError struct {
	error
	after time.Duration
}

// After returns the time period after which the operation that caused the error
// should be retried.
func (r retryableError) After() time.Duration {
	return r.after
}

// NewMaybeRetryableAggregate converts a slice of errors into a single error
// value.  Nil values will be filtered from the slice.  If the filtered slice is
// empty, the return value will be nil.  Else, if any values are non-retryable
// errors, the result will be an Aggregate interface.  Else, if all errors are
// retryable, the result will be a retryable Error interface, with After() equal
// to the minimum of all the errors' After() values.
func NewMaybeRetryableAggregate(errs []error) error {
	aggregate := utilerrors.NewAggregate(errs)
	if aggregate == nil {
		return nil
	}
	afterHasInitialValue := false
	var after time.Duration
	for _, err := range aggregate.Errors() {
		switch e := err.(type) {
		case Error:
			if !afterHasInitialValue || e.After() < after {
				after = e.After()
			}
			afterHasInitialValue = true
		default:
			return aggregate
		}
	}
	return New(aggregate, after)
}
