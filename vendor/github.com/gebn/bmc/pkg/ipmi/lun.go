package ipmi

import (
	"fmt"
)

// LUN represents a Logical Unit Number. It can be thought of as a sub-address
// within a given slave address (i.e. a sub-interface of the BMC, reachable via
// IPMB). It is meaningless in the context of software IDs. This is a two-bit
// field. See section 7.2 of IPMI v1.5 and v2.0 for value definitions. It is a
// 2-bit uint on the wire.
type LUN uint8

const (
	LUNBMC LUN = 0x0 // both requests and responses
	LUNSMS LUN = 0x2
)

func (l LUN) String() string {
	switch l {
	case 0:
		return "0(BMC command/event request message)"
	case 1:
		return "1(OEM 1)"
	case 2:
		return "2(SMS command)"
	case 3:
		return "3(OEM 2)"
	default:
		return fmt.Sprintf("%v(Invalid)", uint8(l))
	}
}
