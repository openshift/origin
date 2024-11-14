package roundtripper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/google/uuid"
	"k8s.io/apiserver/pkg/apis/audit"

	"github.com/openshift/origin/pkg/disruption/backend"
)

// Config holds the user specified parameters to make a new client
type Config struct {
	RT            http.RoundTripper
	ClientTimeout time.Duration
	UserAgent     string

	// EnableShutdownResponseHeader indicates whether to include the shutdown
	// response header extractor, this should be true only when the
	// request(s) are being sent to the kube-apiserver.
	EnableShutdownResponseHeader bool

	// HostNameDecoder, if specified, is used to decode the APIServerIdentity
	// inside the shutdown response header into the actual human readable hostname.
	HostNameDecoder backend.HostNameDecoder

	// Source contains pod name if incluster monitor is used
	Source string
}

// NewClient returns a new Client instance constructed
// from the user specified configuration.
func NewClient(c Config) (backend.Client, error) {
	if c.RT == nil {
		return nil, fmt.Errorf("must initialize a round tripper before setting up a client")
	}

	client := &http.Client{
		Transport: c.RT,
		Timeout:   c.ClientTimeout,
	}
	return WrapClient(client, c.ClientTimeout, c.UserAgent, c.EnableShutdownResponseHeader, c.HostNameDecoder, c.Source), nil
}

// WrapClient wraps the base http.Client object
func WrapClient(client *http.Client, timeout time.Duration, userAgent string, shutdownResponse bool, decoder backend.HostNameDecoder, source string) backend.Client {
	// This is the preferred order:
	//   - WithTimeout will set a timeout within which the entire chain should finish
	//   - WithShutdownResponseHeaderExtractor opts in for shutdown response header
	//   - WithAuditID attaches an audit ID to the request header
	//   - WithUserAgent sets the user agent
	//   - WithSource sets the request source
	//   - WithGotConnTrace sets the connection trace
	//   - http.Client.Do executes
	//   - WithRoundTripLatencyTracking measures the latency of http.Client
	//   - WithResponseBodyReader reads off the response body
	//   - WithShutdownResponseHeaderExtractor parses the shutdown response header
	c := WithRoundTripLatencyTracking(client)
	c = WithResponseBodyReader(c)
	c = WithGotConnTrace(c)
	c = WithSource(c, source)
	c = WithUserAgent(c, userAgent)
	c = WithAuditID(c)
	if shutdownResponse {
		c = WithShutdownResponseHeaderExtractor(c, decoder)
	}
	c = WithTimeout(c, timeout)

	return c
}

// WithAuditID generates an audit ID and attaches it to the
// appropriate request header.
func WithAuditID(delegate backend.Client) backend.Client {
	return backend.ClientFunc(func(req *http.Request) (*http.Response, error) {
		uid := uuid.New().String()
		req.Header.Set(audit.HeaderAuditID, uid)
		return delegate.Do(req)
	})
}

// WithGotConnTrace attaches a 'GotConn' and 'DNSDone' client
// trace to the given request.
//
//	 GotConn: this client trace is called after a successful connection is
//		  obtained, using this trace we can infer whether this connection has
//		  been previously used for another HTTP request.
//	 DNSDone: this client trace is called when a DNS lookup ends, and we
//	   can obtain the error that occurred during the DNS lookup, if any.
//
// This function will attach the data obtained from the client trace
// to the request context so it can be retrieved later.
func WithGotConnTrace(delegate backend.Client) backend.Client {
	// TODO: for now this is a global lock that applies to each disruption test
	//  instance, we can use a more fine grained request scoped lock in the
	//  future since the data race can happen at the http.Request level only.
	lock := sync.Mutex{}

	return backend.ClientFunc(func(req *http.Request) (*http.Response, error) {
		trace := &httptrace.ClientTrace{
			GotConn: func(ci httptrace.GotConnInfo) {
				connInfo := &backend.GotConnInfo{}
				connInfo.RemoteAddr = ci.Conn.RemoteAddr().String()
				connInfo.Reused = ci.Reused
				connInfo.IdleTime = ci.IdleTime
				connInfo.WasIdle = ci.WasIdle

				lock.Lock()
				if data := backend.RequestContextAssociatedDataFrom(req.Context()); data != nil {
					data.GotConnInfo = connInfo
				}
				lock.Unlock()
			},
			DNSDone: func(d httptrace.DNSDoneInfo) {
				lock.Lock()
				if data := backend.RequestContextAssociatedDataFrom(req.Context()); data != nil {
					data.DNSErr = d.Err
				}
				lock.Unlock()
			},
		}
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
		return delegate.Do(req)
	})
}

// WithUserAgent sets the given 'agent' as the User Agent for this
// request if the 'User-Agent' request header is not already set.
func WithUserAgent(delegate backend.Client, agent string) backend.Client {
	return backend.ClientFunc(func(req *http.Request) (*http.Response, error) {
		if len(req.Header.Get("User-Agent")) != 0 {
			return delegate.Do(req)
		}

		req.Header.Set("User-Agent", agent)
		return delegate.Do(req)
	})
}

// WithResponseBodyReader makes an attempt to read the body of the response
// from the server, any error that occurs while reading the body,
// or while closing the underlying stream will be saved for later access.
func WithResponseBodyReader(delegate backend.Client) backend.Client {
	return backend.ClientFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := delegate.Do(req)
		if err != nil {
			return resp, err
		}

		var (
			body        []byte
			bodyReadErr error
		)
		body, bodyReadErr = io.ReadAll(resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			switch {
			case bodyReadErr == nil:
				bodyReadErr = fmt.Errorf("error while closing response body: %w", closeErr)
			default:
				bodyReadErr = fmt.Errorf("response body read err: %w, response body close err: %v", bodyReadErr, closeErr)
			}
		}

		if data := backend.RequestContextAssociatedDataFrom(req.Context()); data != nil {
			data.ResponseBody = body
			data.ResponseBodyReadErr = bodyReadErr
		}
		return resp, err
	})
}

// WithRoundTripLatencyTracking will measure the round trip latency incurred
// and save it to the context so it can be accessed later.
func WithRoundTripLatencyTracking(delegate backend.Client) backend.Client {
	return backend.ClientFunc(func(req *http.Request) (*http.Response, error) {
		startedAt := time.Now()
		defer func() {
			duration := time.Since(startedAt)
			if data := backend.RequestContextAssociatedDataFrom(req.Context()); data != nil {
				data.RoundTripDuration = duration
			}
		}()
		return delegate.Do(req)
	})
}

// WithTimeout will create a context with deadline that is proportional
// to the given timeout and assign it to the request.
func WithTimeout(delegate backend.Client, timeout time.Duration) backend.Client {
	return backend.ClientFunc(func(req *http.Request) (*http.Response, error) {
		if timeout > 0 {
			// from the existing backend sampler:
			//   this is longer than the http client timeout to avoid tripping,
			//   but is here to be sure we finish eventually
			ctx, cancel := context.WithTimeout(req.Context(), timeout*3/2)
			defer cancel()
			req = req.WithContext(ctx)
		}

		return delegate.Do(req)
	})
}

// WithSource sets pod name for incluster monitor.
func WithSource(delegate backend.Client, source string) backend.Client {
	return backend.ClientFunc(func(req *http.Request) (*http.Response, error) {
		defer func() {
			if data := backend.RequestContextAssociatedDataFrom(req.Context()); data != nil {
				data.Source = source
			}
		}()
		return delegate.Do(req)
	})
}
