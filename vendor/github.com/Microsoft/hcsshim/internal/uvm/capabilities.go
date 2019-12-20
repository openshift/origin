package uvm

import "github.com/Microsoft/hcsshim/internal/schema1"

// SignalProcessSupported returns `true` if the guest supports the capability to
// signal a process.
//
// This support was added RS5+ guests.
func (uvm *UtilityVM) SignalProcessSupported() bool {
	if props, err := uvm.hcsSystem.Properties(schema1.PropertyTypeGuestConnection); err == nil {
		return props.GuestConnectionInfo.GuestDefinedCapabilities.SignalProcessSupported
	}
	return false
}
