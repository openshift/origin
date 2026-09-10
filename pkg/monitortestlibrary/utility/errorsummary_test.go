package utility

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"syscall"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// internalAPIHost stands in for the address that must never reach a publicly
// archived CI log.
const internalAPIHost = "api-int.mycluster.example.com"

func urlError(op string, cause error) error {
	return &url.Error{
		Op:  op,
		URL: "https://" + net.JoinHostPort(internalAPIHost, "6443") + "/api/v1/pods",
		Err: cause,
	}
}

func TestScrapeErrorSummary(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil",
			err:  nil,
			want: "<nil>",
		},
		{
			name: "apiserver rejection keeps its code and reason",
			err:  apierrors.NewServiceUnavailable("apiserver is restarting"),
			want: "apiserver status 503 ServiceUnavailable",
		},
		{
			name: "wrapped apiserver rejection is still recognized",
			err: fmt.Errorf("couldn't list pods: %w",
				apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, "etcd-operator-xyz")),
			want: "apiserver status 404 NotFound",
		},
		{
			name: "connection refused during a kubelet restart",
			err:  urlError("Get", syscall.ECONNREFUSED),
			want: "connection refused",
		},
		{
			name: "connection reset during a kubelet restart",
			err:  urlError("Get", syscall.ECONNRESET),
			want: "connection reset",
		},
		{
			name: "unclassified transport failure keeps only the verb and cause type",
			err:  urlError("Post", errors.New("http2: server sent GOAWAY")),
			want: "Post request failed: *errors.errorString",
		},
		{
			name: "joined errors are summarized one by one",
			err: errors.Join(
				apierrors.NewServiceUnavailable("apiserver is restarting"),
				urlError("Get", syscall.ECONNREFUSED),
			),
			want: "2 errors: apiserver status 503 ServiceUnavailable; connection refused",
		},
		{
			name: "opaque error falls back to its root cause type",
			err:  fmt.Errorf("error reading log for pods/etcd-operator-xyz: %w", errors.New("container is terminated")),
			want: "*errors.errorString",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorSummary(tt.err)
			if got != tt.want {
				t.Errorf("ErrorSummary() = %q, want %q", got, tt.want)
			}
			if strings.Contains(got, internalAPIHost) {
				t.Errorf("ErrorSummary() leaked the apiserver address: %q", got)
			}
		})
	}
}

// TestScrapeErrorSummaryNeverEchoesTheRequestURL guards the property the summary
// exists for: however an error is nested, its rendering must not carry the URL
// that the underlying *url.Error prints.
func TestScrapeErrorSummaryNeverEchoesTheRequestURL(t *testing.T) {
	transportErr := urlError("Get", syscall.ECONNREFUSED)
	nestings := []error{
		transportErr,
		fmt.Errorf("couldn't list pods: %w", transportErr),
		errors.Join(transportErr, transportErr),
		fmt.Errorf("unable to scan operator logs: %w", errors.Join(transportErr)),
	}

	for _, err := range nestings {
		if got := ErrorSummary(err); strings.Contains(got, internalAPIHost) {
			t.Errorf("ErrorSummary(%T) leaked the apiserver address: %q", err, got)
		}
		// Guard the premise: the raw rendering really does expose the address.
		if !strings.Contains(err.Error(), internalAPIHost) {
			t.Fatalf("test case %T no longer exercises a leaky error", err)
		}
	}
}
