package ipmi

import (
	"fmt"
)

// SlaveAddress is a 7-bit I2C slave address. The requester and responder of
// IPMB messages is always a slave address; in LAN messages, the addresses can
// also be software IDs.
type SlaveAddress uint8

const (
	// SlaveAddressBMC is the address of the BMC. Accounting for the 0 slave
	// address bit, this is equivalent to an address of 0x20.
	SlaveAddressBMC SlaveAddress = 0x10
)

// Address converts a slave address into a value suitable for inclusion in an
// IPMI message.
func (s SlaveAddress) Address() Address {
	return Address(uint8(s) << 1)
}

func (s SlaveAddress) String() string {
	// there are no semantics here like there are with slave addresses; we just
	// special-case the BMC itself for readability
	switch s {
	case SlaveAddressBMC:
		return "BMC"
	default:
		return fmt.Sprintf("%#x", uint8(s))
	}
}
