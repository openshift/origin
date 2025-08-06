package ipmi

import (
	"fmt"
)

// StatusCode represents an RMCP+ status code. A value of this type is contained
// in the RMCP+ Open Session Response and RAKP Messages 2, 3 and 4. This is the
// equivalent of an IPMI completion code. See section 13.24 for the full list of
// definitions.
type StatusCode uint8

const (
	// StatusCodeOK indicates successful completion, absent of error. This can
	// exist in all message types.
	StatusCodeOK StatusCode = iota

	// StatusCodeInsufficientResources indicates there were insufficient
	// resources to create a session. This can exist in all message types.
	StatusCodeInsufficientResources

	// StatusCodeInvalidSessionID indicates the managed system or remote console
	// does not recognise the session ID sent by the other end. In practice, the
	// remote console will likely be at fault. This can exist in all message
	// types.
	StatusCodeInvalidSessionID

	// StatusCodeUnauthorisedName is sent in RAKP Message 2 to indicate the
	// username was not found in the BMC's users table.
	StatusCodeUnauthorisedName StatusCode = 0x0d

	// StatusCodeUnsupportedCipherSuite is sent in RMCP+ Open Session Response
	// to indicate the BMC cannot satisfy an acceptable combination of the
	// requested authentication/integrity/encryption parameters.
	StatusCodeUnsupportedCipherSuite StatusCode = 0x11

	// StatusCodeInvalidRequestLength is sent when the request is too short, or
	// too long. This is an IPMI rather than RMCP+ value, however some BMCs
	// return it regardless. It resides in the reserved status code space,
	// which we assume will never be used.
	StatusCodeInvalidRequestLength StatusCode = 0xc7
)

var (
	statusCodeDescriptions = map[StatusCode]string{
		StatusCodeOK:                     "Ok",
		StatusCodeInsufficientResources:  "Insufficient Resources",
		StatusCodeInvalidSessionID:       "Invalid Session ID",
		StatusCodeUnauthorisedName:       "Unauthorised User",
		StatusCodeUnsupportedCipherSuite: "Unsupported Cipher Suite",
		StatusCodeInvalidRequestLength:   "Invalid Request Length",
	}
)

func (s StatusCode) Description() string {
	if description, ok := statusCodeDescriptions[s]; ok {
		return description
	}
	return "Unknown"
}

// IsTemporary returns whether the code indicates a retry may produce a
// successful result, or the error is permanent.
func (s StatusCode) IsTemporary() bool {
	return s == StatusCodeInsufficientResources
}

func (s StatusCode) String() string {
	return fmt.Sprintf("%#.2x(%v)", uint8(s), s.Description())
}
