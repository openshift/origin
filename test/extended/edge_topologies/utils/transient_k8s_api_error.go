package utils

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"syscall"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

// IsTransientKubernetesAPIError reports whether err is a likely client/network flake talking to the API
// (timeouts, connection loss, overloaded apiserver). Callers may log, sleep, and retry or proceed without failing the test.
func IsTransientKubernetesAPIError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if utilnet.IsConnectionRefused(err) || utilnet.IsConnectionReset(err) || utilnet.IsTimeout(err) ||
		utilnet.IsHTTP2ConnectionLost(err) || utilnet.IsProbableEOF(err) {
		return true
	}
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
		switch code := int(statusErr.Status().Code); code {
		case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		default:
		}
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.ETIMEDOUT, syscall.EPIPE:
			return true
		}
	}
	var ne net.Error
	if errors.As(err, &ne) && ne != nil && ne.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "tls handshake timeout") ||
		strings.Contains(msg, "no route to host") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "http2: client connection lost") ||
		strings.Contains(msg, "failed to connect")
}
