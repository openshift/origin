package ipmi

import (
	"fmt"
)

// CompletionCode indicates whether a command executed successfully. It is
// analogous to a command status code. It is a 1 byte uint on the wire. Values
// are specified in Table 5-2 of the IPMI v2.0 spec.
//
// N.B. if the completion code is not 0, the rest of the response may be
// truncated, and if it is not, the remaining structure is OEM-dependent, so in
// practice the rest of the message should be uninterpreted.
type CompletionCode uint8

const (
	CompletionCodeNormal CompletionCode = 0x0

	// CompletionCodeInvalidSessionID is returned by Close Session if the
	// specified session ID does not match one the BMC knows about. Whether
	// this is also returned if the used doesn't have the required privileges
	// is untested.
	CompletionCodeInvalidSessionID CompletionCode = 0x87

	CompletionCodeNodeBusy            CompletionCode = 0xc0
	CompletionCodeUnrecognisedCommand CompletionCode = 0xc1
	CompletionCodeTimeout             CompletionCode = 0xc3

	// CompletionCodeReservationCanceledOrInvalid means that either the
	// requester's reservation has been canceled or the request's reservation
	// ID is invalid.
	CompletionCodeReservationCanceledOrInvalid CompletionCode = 0xc5

	// CompletionCodeRequestTruncated means the request ended prematurely. Did
	// you forget to add the final request data layer?
	CompletionCodeRequestTruncated CompletionCode = 0xc6

	// CompletionCodeInsufficientPrivileges indicates the channel or effective
	// user privilege level is insufficient to execute the command, or the
	// request was blocked by the firmware firewall.
	CompletionCodeInsufficientPrivileges CompletionCode = 0xd4

	CompletionCodeUnspecified CompletionCode = 0xff
)

var (
	completionCodeDescriptions = map[CompletionCode]string{
		CompletionCodeNormal:                 "Normal",
		CompletionCodeInvalidSessionID:       "Invalid Session ID",
		CompletionCodeNodeBusy:               "Node Busy",
		CompletionCodeUnrecognisedCommand:    "Unrecognised Command",
		CompletionCodeTimeout:                "Timeout",
		CompletionCodeRequestTruncated:       "Request Truncated",
		CompletionCodeInsufficientPrivileges: "Insufficient Privileges",
		CompletionCodeUnspecified:            "Unspecified Error",
	}
)

func (c CompletionCode) Description() string {
	if description, ok := completionCodeDescriptions[c]; ok {
		return description
	}
	return "Unknown"
}

// IsTemporary returns whether the code indicates a retry may produce a
// successful result, or the error is permanent.
func (c CompletionCode) IsTemporary() bool {
	// at some point, will be more efficient implemented as
	// map[CompletionCode]struct{}, but this is sufficient for now
	return c == CompletionCodeNodeBusy || c == CompletionCodeTimeout
}

func (c CompletionCode) String() string {
	return fmt.Sprintf("%#.2x(%v)", uint8(c), c.Description())
}
