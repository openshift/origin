package utility

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

// ErrorSummary describes err for a test log without its transport detail.
//
// Kubernetes client errors wrap *url.Error, whose text repeats the whole request
// URL, so logging one verbatim writes the cluster's internal apiserver address
// into publicly archived CI artifacts. Only the classification is reported: an
// apiserver status code, a named connection failure, or the concrete type of the
// root cause.
func ErrorSummary(err error) string {
	if err == nil {
		return "<nil>"
	}

	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		inner := joined.Unwrap()
		summaries := make([]string, 0, len(inner))
		for _, e := range inner {
			summaries = append(summaries, ErrorSummary(e))
		}
		return fmt.Sprintf("%d errors: %s", len(inner), strings.Join(summaries, "; "))
	}

	var statusErr apierrors.APIStatus
	if errors.As(err, &statusErr) {
		status := statusErr.Status()
		return fmt.Sprintf("apiserver status %d %s", status.Code, status.Reason)
	}

	switch {
	case utilnet.IsConnectionRefused(err):
		return "connection refused"
	case utilnet.IsConnectionReset(err):
		return "connection reset"
	case utilnet.IsTimeout(err):
		return "timeout"
	}

	// url.Error.Error() is the one that spells out the request URL; its verb and
	// its cause are safe to keep.
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return fmt.Sprintf("%s request failed: %T", urlErr.Op, urlErr.Err)
	}

	// Any other message may embed a URL too, so report the concrete type of the
	// root cause rather than its text.
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return fmt.Sprintf("%T", err)
		}
		err = unwrapped
	}
}
