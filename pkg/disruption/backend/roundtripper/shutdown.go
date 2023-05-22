package roundtripper

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/disruption/backend"
)

// WithShutdownResponseHeaderExtractor adds the 'X-Openshift-If-Disruption'
// request header to the given request in order to opt in to receive the
// shutdown response header; upon receiving a response from the server it
// parses the 'X-OpenShift-Disruption' response header and attaches the
// parsed data to the request context.
//
//	format: shutdown=%t shutdown-delay-duration=%s elapsed=%s host=%s
func WithShutdownResponseHeaderExtractor(delegate backend.Client, decoder backend.HostNameDecoder) backend.Client {
	return backend.ClientFunc(func(req *http.Request) (*http.Response, error) {
		req.Header.Set("X-Openshift-If-Disruption", "true")

		resp, err := delegate.Do(req)
		if err != nil {
			return resp, err
		}
		scoped := backend.RequestContextAssociatedDataFrom(req.Context())
		if scoped == nil {
			return resp, err
		}

		h := resp.Header.Get("X-Openshift-Disruption")
		if len(h) == 0 {
			scoped.ShutdownResponseHeaderParseErr = fmt.Errorf("expected X-Openshift-Disruption response header")
			return resp, err
		}

		shutdown, err := parse(h)
		if shutdown != nil && decoder != nil {
			shutdown.Hostname = decoder.Decode(shutdown.Hostname)
		}
		scoped.ShutdownResponseHeaderParseErr = err
		scoped.ShutdownResponse = shutdown
		return resp, err
	})
}

func parse(csv string) (*backend.ShutdownResponse, error) {
	var (
		shutdown bool
		duration string
		elapsed  string
		host     string
	)
	reader := strings.NewReader(csv)
	_, err := fmt.Fscanf(reader, "shutdown=%t shutdown-delay-duration=%s elapsed=%s host=%s",
		&shutdown, &duration, &elapsed, &host)
	if err != nil {
		return nil, err
	}

	shutdownDelayDuration, err := time.ParseDuration(duration)
	if err != nil {
		return nil, err
	}
	elapsedDuration, err := time.ParseDuration(elapsed)
	if err != nil {
		return nil, err
	}

	return &backend.ShutdownResponse{
		ShutdownInProgress:    shutdown,
		ShutdownDelayDuration: shutdownDelayDuration,
		Elapsed:               elapsedDuration,
		Hostname:              host,
	}, nil
}
