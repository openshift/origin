package ipmi

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// PowerRestorePolicy indicates what the chassis is configured to do when mains
// power returns. This is a 2-bit uint on the wire.
type PowerRestorePolicy uint8

const (
	// PowerRestorePolicyRemainOff means the system will not attempt to turn on
	// when power returns, regardless of state before the outage.
	PowerRestorePolicyRemainOff PowerRestorePolicy = iota

	// PowerRestorePolicyPriorState means the system will return to the state it
	// was in when the power was lost.
	PowerRestorePolicyPriorState

	// PowerRestorePolicyPowerOn means the system will always attempt to turn on
	// when power returns, regardless of state before the outage.
	PowerRestorePolicyPowerOn

	// PowerRestorePolicyUnknown means the BMC does not know what the chassis
	// will do.
	PowerRestorePolicyUnknown
)

// Description returns a human-readable representation of the policy.
func (p PowerRestorePolicy) Description() string {
	switch p {
	case PowerRestorePolicyRemainOff:
		return "Remain off"
	case PowerRestorePolicyPriorState:
		return "Return to prior state"
	case PowerRestorePolicyPowerOn:
		return "Power on"
	default:
		return "Unknown"
	}
}

func (p PowerRestorePolicy) String() string {
	return fmt.Sprintf("%v(%v)", uint8(p), p.Description())
}

// ChassisIdentifyState indicates the current state of the chassis
// identification mechanism, usually a flashing light.
type ChassisIdentifyState uint8

const (
	// ChassisIdentifyStateOff means the chassis identification mechanism is
	// not currently active.
	ChassisIdentifyStateOff ChassisIdentifyState = iota

	// ChassisIdentifyStateTemporary means the chassis identification mechanism
	// is active, but will disable automatically at an unknown point in the
	// future.
	ChassisIdentifyStateTemporary

	// ChassisIdentifyStateIndefinite means the chassis identification mechanism
	// will remain active until manually disabled.
	ChassisIdentifyStateIndefinite

	// ChassisIdentifyStateUnknown means the BMC indicated it does not support
	// revealing the identify state in the Get Chassis Status command, however
	// it may still be supported via other means - issue a Get Command Support
	// command to find out.
	ChassisIdentifyStateUnknown ChassisIdentifyState = 0xff
)

// Description returns a human-readable representation of the state.
func (s ChassisIdentifyState) Description() string {
	switch s {
	case ChassisIdentifyStateOff:
		return "Off"
	case ChassisIdentifyStateTemporary:
		return "On temporarily"
	case ChassisIdentifyStateIndefinite:
		return "On indefinitely"
	default:
		return "Unknown"
	}
}

func (s ChassisIdentifyState) String() string {
	return fmt.Sprintf("%v(%v)", uint8(s), s.Description())
}

// GetChassisStatusRsp represents the managed system's response to a Get Chassis
// Status command, specified in 22.2 and 28.2 of IPMI v1.5 and v2.0
// respectively.
type GetChassisStatusRsp struct {
	layers.BaseLayer

	// PowerRestorePolicy indicates what the system will do if power is lost
	// then returns.
	PowerRestorePolicy PowerRestorePolicy

	// PowerControlFault indicates whether the last attempt to change the system
	// power state failed.
	PowerControlFault bool

	// PowerFault indicates whether a fault has been detected in the main power
	// subsystem.
	PowerFault bool

	// Interlock indicates whether the last system shutdown was caused by the
	// activation of a chassis panel interlock switch.
	Interlock bool

	// PowerOverload indicates whether the last system shutdown was caused by a
	// power overload.
	PowerOverload bool

	// PoweredOn indicates whether the system power is on. If false, the system
	// could be in S4/S5, or mechanical off.
	PoweredOn bool

	// PoweredOnByIPMI indicates whether the last command to turn on the system
	// was issued via IPMI.
	PoweredOnByIPMI bool

	// considered having a LastPowerDownCause struct, however the spec suggests
	// these states are not mutually exclusive.

	// LastPowerDownFault indicates whether the last power down was caused by a
	// power fault.
	LastPowerDownFault bool

	// LastPowerDownInterlock indicates whether the last power down was caused
	// by the activation of a chassis panel interlock switch.
	LastPowerDownInterlock bool

	// LastPowerDownOverload indicates whether the last power down was caused by
	// a power overload.
	LastPowerDownOverload bool

	// LastPowerDownSupplyFailure indicates whether the last power down was
	// caused by an interruption in mains power to the system (not a PSU
	// failure).
	LastPowerDownSupplyFailure bool

	// ChassisIdentifyState indicates the current state of the chassis
	// identification mechanism.
	ChassisIdentifyState ChassisIdentifyState

	// CoolingFault indicates whether a cooling or fan fault has been detected.
	CoolingFault bool

	// DriveFault indicates whether a disk drive in the system is faulty.
	DriveFault bool

	// Lockout indicates whether both power off and reset via the chassis
	// buttons is disabled.
	Lockout bool

	// Intrusion indicates a chassis intrusion is active.
	Intrusion bool

	// StandbyButtonDisableAllowed indicates whether the chassis allows
	// disabling the standby/sleep button.
	StandbyButtonDisableAllowed bool

	// DiagnosticInterruptButtonDisableAllowed indicates whether the chassis
	// allows disabling the diagnostic interrupt button.
	DiagnosticInterruptButtonDisableAllowed bool

	// ResetButtonDisableAllowed indicates whether the chassis allows disabling
	// the reset button.
	ResetButtonDisableAllowed bool

	// PowerOffButtonDisableAllowed indicates whether the chassis allows
	// disabling the power off button. If the button also controls sleep, this
	// being true indicates that sleep requests via the same button can also be
	// disabled.
	PowerOffButtonDisableAllowed bool

	// StandbyButtonDisabled indicates whether the chassis' standby/sleep
	// button is currently disabled.
	StandbyButtonDisabled bool

	// DiagnosticInterruptButtonDisabled indicates whether the chassis'
	// diagnostic interrupt button is currently disabled.
	DiagnosticInterruptButtonDisabled bool

	// ResetButtonDisableAllowed indicates whether the chassis' reset button is
	// currently disabled.
	ResetButtonDisabled bool

	// PowerOffButtonDisabled indicates whether the chassis' power off button
	// is currently disabled. If the button also controls sleep, this being true
	// indicates that sleep requests via the same button are also disabled.
	PowerOffButtonDisabled bool
}

func (*GetChassisStatusRsp) LayerType() gopacket.LayerType {
	return LayerTypeGetChassisStatusRsp
}

func (s *GetChassisStatusRsp) CanDecode() gopacket.LayerClass {
	return s.LayerType()
}

func (*GetChassisStatusRsp) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (s *GetChassisStatusRsp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	// min length is 3 bytes, if Front Panel Button Capabilities are not
	// supported
	if len(data) < 3 {
		df.SetTruncated()
		return fmt.Errorf("response must be 3 or 4 bytes, got %v", len(data))
	}

	s.PowerRestorePolicy = PowerRestorePolicy((data[0] & 0x60) >> 5)
	s.PowerControlFault = data[0]&(1<<4) != 0
	s.PowerFault = data[0]&(1<<3) != 0
	s.Interlock = data[0]&(1<<2) != 0
	s.PowerOverload = data[0]&(1<<1) != 0
	s.PoweredOn = data[0]&1 != 0

	s.PoweredOnByIPMI = data[1]&(1<<4) != 0
	s.LastPowerDownFault = data[1]&(1<<3) != 0
	s.LastPowerDownInterlock = data[1]&(1<<2) != 0
	s.LastPowerDownOverload = data[1]&(1<<1) != 0
	s.LastPowerDownSupplyFailure = data[1]&1 != 0

	if data[2]&(1<<6) != 0 {
		s.ChassisIdentifyState = ChassisIdentifyState((data[2] & 0x30) >> 4)
	} else {
		s.ChassisIdentifyState = ChassisIdentifyStateUnknown
	}
	s.CoolingFault = data[2]&(1<<3) != 0
	s.DriveFault = data[2]&(1<<2) != 0
	s.Lockout = data[2]&(1<<1) != 0
	s.Intrusion = data[2]&1 != 0

	if len(data) > 3 {
		s.StandbyButtonDisableAllowed = data[3]&(1<<7) != 0
		s.DiagnosticInterruptButtonDisableAllowed = data[3]&(1<<6) != 0
		s.ResetButtonDisableAllowed = data[3]&(1<<5) != 0
		s.PowerOffButtonDisableAllowed = data[3]&(1<<4) != 0
		s.StandbyButtonDisabled = data[3]&(1<<3) != 0
		s.DiagnosticInterruptButtonDisabled = data[3]&(1<<2) != 0
		s.ResetButtonDisabled = data[3]&(1<<1) != 0
		s.PowerOffButtonDisabled = data[3]&1 != 0

		s.BaseLayer.Contents = data[:4]
		s.BaseLayer.Payload = data[4:]
	} else {
		s.StandbyButtonDisableAllowed = false
		s.DiagnosticInterruptButtonDisableAllowed = false
		s.ResetButtonDisableAllowed = false
		s.PowerOffButtonDisableAllowed = false
		s.StandbyButtonDisabled = false
		s.DiagnosticInterruptButtonDisabled = false
		s.ResetButtonDisabled = false
		s.PowerOffButtonDisabled = false

		s.BaseLayer.Contents = data[:3]
		s.BaseLayer.Payload = data[3:]
	}
	return nil
}

type GetChassisStatusCmd struct {
	Rsp GetChassisStatusRsp
}

// Name returns "Get Chassis Status".
func (*GetChassisStatusCmd) Name() string {
	return "Get Chassis Status"
}

// Operation returns OperationGetChassisStatusReq.
func (*GetChassisStatusCmd) Operation() *Operation {
	return &OperationGetChassisStatusReq
}

func (*GetChassisStatusCmd) RemoteLUN() LUN {
	return LUNBMC
}

func (*GetChassisStatusCmd) Request() gopacket.SerializableLayer {
	return nil
}

func (c *GetChassisStatusCmd) Response() gopacket.DecodingLayer {
	return &c.Rsp
}
