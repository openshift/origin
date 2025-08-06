package bmc

import (
	"context"

	"github.com/gebn/bmc/pkg/ipmi"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// we care less about version here - distribution will follow connections
	// unless the user is treating different versions differently, in which case
	// they probably don't care about the break-down

	// we could add authentication, integrity and confidentiality labels to a
	// new algorithms counter, however that will remain static for a given fleet
	// - if people are interested in algorithm support, this is better
	// discovered via infrequent sweeps

	// we could time session establishment, however do we really care, provided
	// it succeeds? would also be a very sparse histogram

	// session re-opens must be tracked by the user of the library; we don't
	// have any visibility here (at least not currently)

	sessionOpenAttempts = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "session",
		Name:      "open_attempts_total",
		Help:      "The number of times session establishment has begun.",
	})
	sessionOpenFailures = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: "session",
		Name:      "open_failures_total",
		Help: "The number of times session establishment did not produce " +
			"a usable session-based connection.",
	})
	sessionsOpen = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: "sessions",
		Name:      "open",
		Help: "The number of sessions currently established. We regard " +
			"sessions that failed to close cleanly as closed.",
	})
)

// Session is an established session-based IPMI v1.5 or 2.0 connection. More
// specifically, it is a multi-session connection, as the spec demands this of
// the LAN interface. Commands are sent in the context of the session, using
// the negotiated integrity and confidentiality algorithms.
type Session interface {
	Connection
	SessionCommands

	// ID returns our identifier for this session. Note the managed system and
	// remote console share the same identifier in IPMI v1.5, however each
	// chooses its own identifier in v2.0, so they likely differ.
	ID() uint32

	// Close closes the session by sending a Close Session command to the BMC.
	// As the underlying transport/socket is used but not managed by
	// connections, it is left open in case the user wants to continue issuing
	// session-less commands or establish a new session. It is envisaged that
	// this call is deferred immediately after successful session establishment.
	// If an error is returned, the session can be assumed to be closed.
	Close(context.Context) error
}

// SessionOpts contains session-establishment options common to IPMI v1.5 and
// 2.0. A value of this type is required to establish a version-agnostic
// session.
type SessionOpts struct {

	// Username is the username of the user to connect as. Only ASCII characters
	// (excluding \0) are allowed, and it cannot be more than 16 characters
	// long.
	Username string

	// Password is the password of the above user, stored on the managed system
	// as either 16 bytes (for v1.5, or to preserve the ability to log in with a
	// v1.5 session in v2.0) or 20 bytes of uninterpreted data (hence why this
	// isn't a string, and "special characters" aren't usually allowed).
	// Passwords shorter than the maximum length are padded with 0x00. This is
	// called K_[UID] in the spec ("the key for the user with ID 'UID'"). Some
	// BMCs have tighter constraints, e.g. Super Micro supports up to 19 chars.
	Password []byte

	// MaxPrivilegeLevel is the upper privilege limit for the session. It
	// defaults to ipmi.PrivilegeLevelHighest for IPMI v2.0, and
	// ipmi.PrivilegeLevelUser for IPMI v1.5, which does not have that value,
	// however the channel or user privilege level limit may further constrain
	// allowed commands for either version.
	//
	// Due to the deliberate inconsistency between IPMI versions, it is always
	// recommended to explicitly set this. Unfortunately it was not possible to
	// default to User for v2.0 as, because Highest has value 0, we cannot
	// distinguish between it being implicitly and explicitly set, so we don't
	// know when to intervene.
	MaxPrivilegeLevel ipmi.PrivilegeLevel

	// timeout is inherited from the session-less connection used to create the
	// session, which also controls the time allowed for each attempt of the
	// session establishment commands
}
