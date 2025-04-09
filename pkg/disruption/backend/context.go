package backend

import (
	"context"
	"fmt"
	"time"
)

type associationKeyType int

const associationKey associationKeyType = iota

// WithRequestContextAssociatedData attaches the specified
// RequestContextAssociatedData instance to the request context.
func WithRequestContextAssociatedData(parent context.Context, data *RequestContextAssociatedData) context.Context {
	if data == nil {
		return parent
	}
	return context.WithValue(parent, associationKey, data)
}

// RequestContextAssociatedDataFrom retrieves the associated
// RequestContextAssociatedData instance from the request context.
func RequestContextAssociatedDataFrom(ctx context.Context) *RequestContextAssociatedData {
	data, _ := ctx.Value(associationKey).(*RequestContextAssociatedData)
	return data
}

// RequestContextAssociatedData holds the data that is associated with the
// request context, each round tripper utilizes this association to attach
// relevant diagnostic data associated with the request.
type RequestContextAssociatedData struct {
	// GotConnInfo is the data obtained from the GotConn client
	// connection trace for this request.
	GotConnInfo *GotConnInfo

	// DNSErr is set if the there was an error during DNS lookup,
	// this is obtained from the DNSDone client connection trace.
	DNSErr error

	// RoundTripDuration is the latency incurred in the
	// round trip for this request.
	RoundTripDuration time.Duration

	// ResponseBody is the stored bytes that was obtained from reading
	// off the body of the response received from the server
	ResponseBody []byte
	// ResponseBodyReadErr is set if any error occurs while reading the
	// body of the response, or while closing the underlying stream.
	ResponseBodyReadErr error

	// ShutdownResponse holds the result of parsing the 'X-OpenShift-Disruption'
	// response header, it will be nil if ShutdownResponseHeaderParseErr is set.
	ShutdownResponse *ShutdownResponse
	// ShutdownResponseHeaderParseErr is set if there was an error parsing the
	// 'X-OpenShift-Disruption' response header
	ShutdownResponseHeaderParseErr error

	// Source contains pod name if incluster monitor is used
	Source string
}

// GotConnInfo similar to net/http GotConnInfo without the connection object
type GotConnInfo struct {
	// RemoteAddr returns the remote network address, if known.
	RemoteAddr string

	// Reused is whether this connection has been previously
	// used for another HTTP request.
	Reused bool

	// WasIdle is whether this connection was obtained from an
	// idle pool.
	WasIdle bool

	// IdleTime reports how long the connection was previously
	// idle, if WasIdle is true.
	IdleTime time.Duration
}

func (ci GotConnInfo) String() string {
	return fmt.Sprintf("reused: %t wasIdle: %t idleTime: %s remote-address: %s", ci.Reused, ci.WasIdle, ci.IdleTime, ci.RemoteAddr)
}
