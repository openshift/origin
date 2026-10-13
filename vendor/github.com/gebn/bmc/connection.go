package bmc

import (
	"context"

	"github.com/gebn/bmc/pkg/ipmi"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	connectionOpenAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "connection",
			Name:      "open_attempts_total",
			Help:      "The number of times a BMC has been dialled.",
		},
		[]string{"version"},
	)
	connectionOpenFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "connection",
			Name:      "open_failures_total",
			Help: "The number of times dialling a BMC resulted in an error " +
				"being returned to the user.",
		},
		[]string{"version"},
	)
	connectionsOpen = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: "connections",
			Name:      "open",
			Help: "The number of sessionless sockets currently open. We regard " +
				"sockets that failed to close cleanly as closed.",
		},
		[]string{"version"},
	)

	// effectively the number of times SendCommand() has been called. we
	// could've added several more labels to this, but chose not to:
	//
	// Version: we probably don't care about this at the command level - the
	// distribution will follow the number of connections, so we track it there,
	// with # open connections per version
	//
	// Connection: do we really care? most commands can only be executed in a
	// session; a given command is likely to always be in a session or outside,
	// never both
	//
	// NetFn: what does this tell us that command name doesn't? Do we really
	// care? This, body code and enterprise would be useful for deduping the
	// name, e.g. if two enterprises had the same command name, but we don't
	// have that problem.
	commandAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "command",
			Name:      "attempts_total",
			Help:      "The number of times a user has asked to send a command.",
		},
		// N.B. collision condition - if two commands from different enterprises
		// or NetFns have the same name, they will be counted as one; can add
		// tie-breaker labels if/when this actually happens; the command name is
		// more there as an indication than forensics
		[]string{"command"}, // e.g. "Get Device ID", specified in Cmd struct
	)

	// serialise and deserialise errors are rolled up into this - to properly
	// diagnose why, we need a level of info only logging can provide. Futile to
	// try to pin this down with metrics, so we don't bother.
	//
	// Note this does not directly correspond to completion codes. If we cannot
	// reach a completion code, that is always a command failure, however a
	// normal completion code can still be a command failure, and a non-normal
	// completion code can be a command success. Command failure is based solely
	// on our ability to send the command and fully decode the response without
	// error. A non-normal completion code is a command failure if and only if
	// the response body could not be fully deserialised. This is correlated
	// with non-normal completion codes, as the BMC tends to truncate it under
	// error conditions, but not directly related. A non-normal completion code
	// that is returned to the user with a nil error is not a failure.
	commandFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "command",
			Name:      "failures_total",
			Help: "The number of times a user has received an error having " +
				"asked to send a command.",
		},
		// we track command name here as well to make this and attempts easily
		// subtractable
		[]string{"command"},
	)

	commandRetries = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "command",
		Name:      "retries_total",
		Help:      "The number of times a given command packet has been re-sent to a BMC, because we did not receive a valid response, if any.",
	})

	// N.B. this is very different from the low-level transport response latency
	// - includes serialise/deserialise, as well as retries
	commandDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "command",
		Name:      "duration_seconds",
		Help:      "The end-to-end time from command send to response return, including retries.",
		Buckets:   prometheus.ExponentialBuckets(0.002, 2.4, 10), // 5.28
	})

	// we don't track the command here, as if commands are failing, we care that
	// they are failing, not about the command - that's for event based metrics.
	commandResponses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "command",
			Name:      "responses_total",
			Help:      "The number of valid command responses received from BMCs.",
		},
		[]string{"code"}, // completion code, printed as text, falling back to hex
	)
)

// Connection is an IPMI v1.5 or v2.0 session-less, single-session or
// multi-session connection. The IPMI version and nature of the connection is
// fixed upon creation - if sending two messages, it will never be the case that
// one uses one wrapper format and the second another. It defines logical things
// that can be done once communication is established with a BMC. Note that this
// is *not* a transport in itself - hence why there is no Close() - but it
// abstracts over a transport to provide its functionality. This interface is
// always wrapped in something else that has a Close() to cleanly terminate the
// underlying connection.
type Connection interface {

	// SendCommand sends a command to the BMC, blocking until it receives a
	// response. This method will retry with the configured per-request timeout
	// until a valid response with a non-temporary error (e.g. resource
	// exhaustion) is received, or the context expires (whichever happens
	// first). If the final request fails with a transport error (including
	// timeout), a serialise/decode error occurs above the command response
	// layer, or the message layer is missing, the returned error will be
	// non-nil, and the completion code must be ignored. If the message layer of
	// the response was decoded successfully, the code will be set to that,
	// however the error can still be non-nil if the command expects a response
	// and that failed to decode correctly.
	//
	// This method uses the response layer (if any) included in the command
	// interface for decoding the response. The caller should first check the
	// error, then the completion code, then assuming both indicate no error,
	// read the response layer if required. The ValidateResponse() function can
	// be used for the sake of brevity.
	//
	// This method does not allocate any memory for layers, so is ideal in
	// situations where you intend to send the same command repeatedly, e.g. a
	// Prometheus exporter. If you don't need this performance, for the sake of
	// one more allocation per command, it is recommended to use the
	// higher-level API, e.g. GetSystemGUID(), which wraps this.
	SendCommand(ctx context.Context, cmd ipmi.Command) (ipmi.CompletionCode, error)

	// Version returns the underlying IPMI version of the connection, either
	// "1.5" or "2.0". Note that even session-less connections use a session
	// wrapper, which has either the v1.5 or v2.0 format. This is provided for
	// informational and debugging purposes - branching based on this value is a
	// code smell.
	Version() string
}
