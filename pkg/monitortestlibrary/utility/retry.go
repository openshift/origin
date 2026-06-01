package utility

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

func IsTransientAPIError(err error) bool {
	if err == nil {
		return false
	}
	if apierrors.IsServerTimeout(err) || apierrors.IsTimeout(err) ||
		apierrors.IsTooManyRequests(err) || apierrors.IsServiceUnavailable(err) ||
		apierrors.IsInternalError(err) {
		return true
	}
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
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
	var lastErr error
	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    5,
		Cap:      16 * time.Second,
	}
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		err := fn()
		if err == nil {
			return true, nil
		}
		lastErr = err
		if IsTransientAPIError(err) {
			klog.Warningf("Transient API error during monitor setup, retrying: %v", err)
			return false, nil
		}
		return false, err
	})
	if err != nil && lastErr != nil && errors.Is(err, wait.ErrWaitTimeout) {
		return lastErr
	}
	return err
}
