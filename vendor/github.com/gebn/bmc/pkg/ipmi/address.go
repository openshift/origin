package ipmi

import (
	"fmt"
)

// Address represents either a slave address or software ID. The LSB is 0 in the
// case of the former, and 1 in the case of the latter.
type Address uint8

func (a Address) IsSlaveAddress() bool {
	return uint8(a)&1 == 0
}

func (a Address) IsSoftwareID() bool {
	return !a.IsSlaveAddress()
}

func (a Address) String() string {
	switch {
	case a.IsSlaveAddress():
		sa := SlaveAddress(a >> 1)
		return fmt.Sprintf("%v(%v)", uint8(sa), sa.String())
	default: // only two cases
		swid := SoftwareID(a >> 1)
		return fmt.Sprintf("%v(%v)", uint8(swid), swid.String())
	}
}
