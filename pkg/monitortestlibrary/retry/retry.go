package retry

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

// Backoff defines the default backoff parameters for transient API server
// errors during monitor test preparation. The preparation phase can race
// against heavy cluster load (e.g. CNV + FRR deployment on metal BGP-virt
// jobs), so we retry on errors that indicate the API server is temporarily
// overloaded.
var Backoff = wait.Backoff{
	Duration: 2 * time.Second,
	Factor:   2.0,
	Jitter:   0.1,
	Steps:    5,
	Cap:      30 * time.Second,
}

// IsTransientAPIError returns true for errors that indicate the API server is
// temporarily unable to handle the request and the operation should be retried.
func IsTransientAPIError(err error) bool {
	if err == nil {
		return false
	}
	// Standard API server overload / timeout responses
	if apierrors.IsServerTimeout(err) || apierrors.IsTimeout(err) ||
		apierrors.IsTooManyRequests(err) || apierrors.IsServiceUnavailable(err) ||
		apierrors.IsInternalError(err) {
		return true
	}
	// Network-level errors: unexpected EOF, connection reset
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}
	// Catch remaining transient error strings from the Go HTTP/2 client and etcd
	errMsg := err.Error()
	for _, substr := range []string{
		"http2: client connection lost",
		"connection reset by peer",
		"etcdserver: request timed out",
		"unexpected EOF",
	} {
		if strings.Contains(errMsg, substr) {
			return true
		}
	}
	return false
}

// OnCreate wraps a Kubernetes resource creation call with exponential backoff
// retry on transient API server errors. This prevents monitor test preparation
// from failing when the API server is under heavy load (e.g. right after CNV
// or FRR deployment).
//
// Usage:
//
//	ns, err := retry.OnCreate(func() (*corev1.Namespace, error) {
//	    return client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
//	})
func OnCreate[T any](fn func() (T, error)) (T, error) {
	var result T
	var lastErr error
	err := wait.ExponentialBackoff(Backoff, func() (bool, error) {
		var createErr error
		result, createErr = fn()
		if createErr == nil {
			return true, nil
		}
		if IsTransientAPIError(createErr) {
			klog.Warningf("Transient API error, retrying: %v", createErr)
			lastErr = createErr
			return false, nil
		}
		// Non-retryable error, stop immediately
		return false, createErr
	})
	if wait.Interrupted(err) {
		return result, fmt.Errorf("timed out retrying after transient errors, last error: %w", lastErr)
	}
	return result, err
}

// OnError wraps any operation with exponential backoff retry on transient API
// server errors. Unlike OnCreate, this works with functions that only return
// an error (no result value).
//
// Usage:
//
//	err := retry.OnError(func() error {
//	    return someOperation(ctx)
//	})
func OnError(fn func() error) error {
	_, err := OnCreate(func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}
