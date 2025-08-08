package ipmi

// PrivilegeLevel dictates which IPMI commands a given user can execute over a
// given channel. Levels are defined in section 6.8 of the IPMI v2.0 spec. On
// the wire, the privilege level is a 4-bit uint.
//
// Each channel and user has an individual privilege level limit, which
// constrains the operations that can be performed via that channel or by that
// user respectively. The lower of the two is the effective limit.
//
// Sessions start at the User privilege level, but can be changed with the Set
// Session Privilege Level command.
type PrivilegeLevel uint8

const (
	// PrivilegeLevelHighest is used in the RMCP+ Open Session Request message
	// to ask the BMC to set the session maximum privilege level to the highest
	// it is willing to, given the cipher suites the remote console indicated
	// support for. Note this is for the channel; the user (provided in RAKP1)
	// may have a lower privilege level limit.
	//
	// Use of this value is not recommended for two reasons. Firstly, you
	// normally know what privilege level you require in advance, and this may
	// result in insufficient privileges, or overly lax ones (breaking the
	// principle of least privilege). Secondly, it is not supported by all BMCs,
	// e.g. Super Micro.
	//
	// This is a reserved value in IPMI v1.5.
	PrivilegeLevelHighest PrivilegeLevel = iota

	PrivilegeLevelCallback
	PrivilegeLevelUser
	PrivilegeLevelOperator
	PrivilegeLevelAdministrator
	PrivilegeLevelOEM
)

func (p PrivilegeLevel) String() string {
	switch p {
	case PrivilegeLevelHighest:
		return "Highest"
	case PrivilegeLevelCallback:
		return "Callback"
	case PrivilegeLevelUser:
		return "User"
	case PrivilegeLevelOperator:
		return "Operator"
	case PrivilegeLevelAdministrator:
		return "Administrator"
	case PrivilegeLevelOEM:
		return "OEM"
	default:
		return "Unknown"
	}
}
