package ipmi

import (
	"fmt"
)

// AuthenticationType is used in the IPMI session header to indicate which
// authentication algorithm was used to sign the message. It is a 4-bit uint on
// the wire.
type AuthenticationType uint8

const (
	AuthenticationTypeNone AuthenticationType = iota
	AuthenticationTypeMD2
	AuthenticationTypeMD5
	_ // reserved
	AuthenticationTypePassword
	AuthenticationTypeOEM
	AuthenticationTypeRMCPPlus // IPMI v2 only
)

func (t AuthenticationType) name() string {
	switch t {
	case AuthenticationTypeNone:
		return "None"
	case AuthenticationTypeMD2:
		return "MD2"
	case AuthenticationTypeMD5:
		return "MD5"
	case AuthenticationTypePassword:
		return "Password/Key"
	case AuthenticationTypeOEM:
		return "OEM"
	case AuthenticationTypeRMCPPlus:
		return "RMCP+"
	default:
		return "Unknown"
	}
}

func (t AuthenticationType) String() string {
	return fmt.Sprintf("%v(%v)", uint8(t), t.name())
}
