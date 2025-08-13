package ipmi

import (
	"fmt"
)

// EntityInstance distinguishes between multiple occurrences of a particular
// entity type in the system, e.g. if it has two processors, their SDRs will
// each have different instance numbers. There are no semantics around the
// "first" instance ID for a given entity, and they are not guaranteed to be
// consecutive. Some manufacturers use the same instance to refer to multiple
// distinct pieces of hardware, e.g. all fans could be behind 29.1. This is a
// 7-bit uint on the wire. See sections 33 and 39 of v1.5 and v2.0 respectively.
//
// Instances from 0x00 through 0x5f are "system-relative", meaning unique within
// the context of the EntityID system-wide. Instances from 0x60 through 0x7f are
// "device-relative", meaning they only have to be unique within the context of
// the EntityID on their management controller (SDR owner). This effectively
// means the component (confusingly, the spec sometimes call this an "entity")
// is now identified by the triple (EntityID, OwnerAddress, EntityInstance)
// rather than (EntityID, EntityInstance). Device-relative entity instances mean
// devices don't have to care about collisions with each other's instance
// numbers, even if they are of the same EntityID.
type EntityInstance uint8

// IsSystemRelative returns whether the instance is guaranteed to be unique for
// the entity ID across all management controllers.
func (i EntityInstance) IsSystemRelative() bool {
	return i <= 0x5f
}

// IsDeviceRelative returns whether the instance is guaranteed to be unique for
// the entity ID only on the same management controller (i.e. owner).
func (i EntityInstance) IsDeviceRelative() bool {
	return i >= 0x60 && i <= 0x7f
}

// String returns a human-readable representation of the instance. The spec
// recommends subtracting 0x60 when displaying device-relative instance values,
// and displaying them with the name of their controller. This implementation
// follows the former (it does not have access to the owner).
func (i EntityInstance) String() string {
	if i.IsSystemRelative() {
		return fmt.Sprintf("%v(System-relative)", uint8(i))
	}
	return fmt.Sprintf("%v(Device-relative)", uint8(i)-0x60)
}
