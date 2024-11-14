package backend

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// RequestResponse holds the given HTTP request, HTTP Response sent from the
// server, and request scoped diagnostic data attached by round trippers
type RequestResponse struct {
	// Request and Response associated with this backend sample.
	// NOTE: the body of the response will have been read and closed
	//  immediately after getting the response from the server.
	// Request may be nil if the sampler fails to create a new request
	// Response may be nil in case the server returned an error
	Request  *http.Request
	Response *http.Response

	// RequestContextAssociatedData holds the data
	// stored in the request context.
	RequestContextAssociatedData
}

func (rr RequestResponse) String() string {
	s := fmt.Sprintf("audit-id=%s conn-reused=%s status-code=%s protocol=%s roundtrip=%s retry-after=%s source=%s",
		rr.GetAuditID(), rr.ConnectionReused(), rr.StatusCode(), rr.Protocol(), rr.RoundTripDuration.Round(time.Millisecond), rr.RetryAfter(), rr.Source)
	if rr.ShutdownResponse != nil {
		s = fmt.Sprintf("%s %s", s, rr.ShutdownResponse.String())
	}
	return s
}

func (rr RequestResponse) Fields() map[string]interface{} {
	fields := map[string]interface{}{}
	fields["audit-id"] = rr.GetAuditID()
	fields["conn-reused"] = rr.ConnectionReused()
	fields["status-code"] = rr.StatusCode()
	fields["protocol"] = rr.Protocol()
	fields["roundtrip"] = rr.RoundTripDuration.Round(time.Millisecond)
	fields["retry-after"] = rr.RetryAfter()
	fields["source"] = rr.Source
	if rr.ShutdownResponse != nil {
		for k, v := range rr.ShutdownResponse.Fields() {
			fields[k] = v
		}
	}

	return fields
}

func (rr RequestResponse) GetAuditID() string {
	if rr.Request != nil {
		return rr.Request.Header.Get("Audit-ID")
	}
	return "<none>"
}

func (rr RequestResponse) ConnectionReused() string {
	if rr.GotConnInfo != nil {
		return strconv.FormatBool(rr.GotConnInfo.Reused)
	}
	return ""
}

func (rr RequestResponse) StatusCode() string {
	if rr.Response != nil {
		return strconv.Itoa(rr.Response.StatusCode)
	}
	return ""
}

func (rr RequestResponse) Protocol() string {
	if rr.Response != nil {
		return rr.Response.Proto
	}
	if rr.Request != nil {
		return rr.Request.Proto
	}
	return "<none>"
}

func (rr RequestResponse) IsRetryAfter() (string, bool) {
	return IsRetryAfter(rr.Response)
}

func (rr RequestResponse) RetryAfter() string {
	resp := rr.Response
	if resp == nil {
		return "<none>"
	}
	seconds, retry := IsRetryAfter(resp)
	if retry {
		return fmt.Sprintf("(%t, %ss)", retry, seconds)
	}
	return "false"
}

// isRetryAfter returns true along with a number of seconds if
// the server instructed us to wait before retrying.
func IsRetryAfter(resp *http.Response) (string, bool) {
	if resp == nil {
		return "", false
	}
	// any 5xx status code and 429 can trigger a retry-after
	if !(resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) {
		return "", false
	}
	// and it must accompany the 'Retry-After' response header
	if h := resp.Header.Get("Retry-After"); len(h) > 0 {
		if _, err := strconv.Atoi(h); err == nil {
			return h, true
		}
	}

	return "", false
}

func (rr RequestResponse) ShutdownInProgress() bool {
	return rr.ShutdownResponse != nil && rr.ShutdownResponse.ShutdownInProgress
}
