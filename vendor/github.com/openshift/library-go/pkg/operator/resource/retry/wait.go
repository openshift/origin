package retry

import (
	"context"

	"k8s.io/apimachinery/pkg/util/wait"
)

// TODO: This should be added to k8s.io/client-go/util/retry

// ConditionWithContextFunc returns true if the condition is satisfied, or an error
// if the loop should be aborted. The context passed to condition function allow function body
// to return faster than context.Done().
type ConditionWithContextFunc func(ctx context.Context) (done bool, err error)

// ExponentialBackoffWithContext repeats a condition check with exponential backoff and stop repeating
// when the context passed to this function is done.
//
// It checks the condition up to Steps times, increasing the wait by multiplying
// the previous duration by Factor.
//
// If Jitter is greater than zero, a random amount of each duration is added
// (between duration and duration*(1+jitter)).
//
// If the condition never returns true, ErrWaitTimeout is returned. All other
// errors terminate immediately.
func ExponentialBackoffWithContext(ctx context.Context, backoff wait.Backoff, condition ConditionWithContextFunc) error {
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		select {
		case <-ctx.Done():
			return false, wait.ErrWaitTimeout
		default:
			return condition(ctx)
		}
	})
}
