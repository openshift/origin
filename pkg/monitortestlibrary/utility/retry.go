package utility

import (
	"context"
	"net/http"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

func IsTransientAPIError(err error) bool {
	if apierrors.IsServerTimeout(err) || apierrors.IsTimeout(err) ||
		apierrors.IsTooManyRequests(err) || apierrors.IsServiceUnavailable(err) ||
		apierrors.IsInternalError(err) {
		return true
	}
	if statusErr, ok := err.(*apierrors.StatusError); ok {
		code := statusErr.Status().Code
		if code == http.StatusGatewayTimeout || code == http.StatusBadGateway {
			return true
		}
	}
	if strings.Contains(err.Error(), "etcdserver: request timed out") {
		return true
	}
	return false
}

func RetryWithExponentialBackoff(ctx context.Context, fn func() error) error {
	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    5,
		Cap:      16 * time.Second,
	}
	return wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		err := fn()
		if err == nil {
			return true, nil
		}
		if IsTransientAPIError(err) {
			klog.Warningf("Transient API error during monitor setup, retrying: %v", err)
			return false, nil
		}
		return false, err
	})
}
