package ipmi

import (
	"fmt"
)

// SoftwareID represents a piece of system software or IPMI event message
// generator. This is a 7-bit uint field.
type SoftwareID uint8

const (
	// SoftwareIDRemoteConsole1 is the software ID of the first remote console.
	// There are 7 in total.
	SoftwareIDRemoteConsole1 SoftwareID = 0x40
)

// Address converts a software ID into an address value suitable for inclusion
// in an IPMI message.
func (s SoftwareID) Address() Address {
	return Address(uint8(s)<<1 | 1)
}

func (s SoftwareID) String() string {
	// see table 5-4 in v1.5 or v2.0 specs
	switch {
	case 0x0 <= s && s <= 0xf:
		return "BIOS"
	case 0x10 <= s && s <= 0x1f:
		return "System Management Interrupt Handler"
	case 0x20 <= s && s <= 0x2f:
		return "System Management Software"
	case 0x30 <= s && s <= 0x3f:
		return "OEM"
	case 0x40 <= s && s <= 0x46:
		// 1-based in the spec
		return fmt.Sprintf("Remote Console #%v", uint8(s)-0x40+1)
	case s == 0x47:
		return "Terminal Mode Remote Console Software"
	default:
		return "Reserved"
	}
}
