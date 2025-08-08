package ipmi

// SessionHandle uniquely identifies an active session within the context of a
// given channel, as opposed to globally for the BMC which is the case for
// SessionID. Each new session receives an incremented handle number. 0x00 is
// ostensibly reserved, or used for single-session channels, however some
// vendors use it as a valid handle number.
type SessionHandle uint8
