package namespace

import (
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

// nsControllerRateLimiter is tuned for a fast recycle time
func nsControllerRateLimiter() workqueue.RateLimiter {
	return workqueue.NewMaxOfRateLimiter(
		// this ensures that we retry namespace deletion at least every minute, never longer.
		workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 60*time.Second),
		// 10 qps, 100 bucket size.  This is only for retry speed and its only the overall factor (not per item)
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)
}
