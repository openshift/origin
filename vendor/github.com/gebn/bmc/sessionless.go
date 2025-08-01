package bmc

// Sessionless is a session-less IPMI v1.5 or 2.0 connection. It enables
// sending commands to a BMC outside the context of a session (however note
// that all such commands can also be validly sent inside a session, for
// example Get Channel Authentication Capabilities is commonly used as a form
// of keepalive). Creating a concrete session-less connection will require a
// transport in order to send bytes.
type Sessionless interface {
	Connection
	SessionlessCommands

	// NewSession() does not go here, as the sessionless interface fixes the
	// session layer, whereas inside a session, this must be manipulated.
	// Creating a session also requires access to a transport, which we
	// deliberately abstract away here.

	// there is no Close() here, as this represents things that can be done over
	// the session rather than the underlying transport, which is kept separate.
	// A session-less connection has no state on the remote console or the
	// managed system, so can simply be abandoned rather than closed.
}
