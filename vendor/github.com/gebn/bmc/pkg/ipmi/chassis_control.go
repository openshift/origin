package ipmi

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// ChassisControl represents a command for the chassis, e.g. power up, or hard
// reset. Possible values are defined by the Chassis Control command in table
// 22-4 and 28-4 of IPMI v1.5 and 2.0 respectively. This is a 4-bit uint on the
// wire.
type ChassisControl uint

const (
	// ChassisControlPowerOff forces the system into a soft off (S4/S5) state.
	// Unlike ChassisControlSoftPowerOff, this does not initiate a clean
	// shutdown of the OS prior to powering down.
	ChassisControlPowerOff ChassisControl = iota

	// ChassisControlPowerOn powers up the chassis.
	ChassisControlPowerOn

	// ChassisControlPowerCycle reboots the machine. The spec recommends that
	// this be a no-op if the system is powered down (S4/S5) and returns a 0xd5
	// completion code, however this command may cause some machines to power
	// up.
	ChassisControlPowerCycle

	// ChassisControlHardReset performs a hardware reset of the chassis,
	// excluding the chassis device itself. For host systems, this corresponds
	// to a system hard reset.
	ChassisControlHardReset

	// ChassisControlDiagnosticInterrupt pulses a diagnostic interrupt to the
	// CPU(s), usually causing a diagnostic dump. The exact interrupt delivered
	// is architecture-dependent.
	ChassisControlDiagnosticInterrupt

	// ChassisControlSoftPowerOff emulates a fatal over-temperature, causing a
	// soft-shutdown of the OS via ACPI. This is not supported by all chassis.
	ChassisControlSoftPowerOff
)

// Description returns a human-readable representation of the command.
func (c ChassisControl) Description() string {
	switch c {
	case ChassisControlPowerOff:
		return "Power off"
	case ChassisControlPowerOn:
		return "Power on"
	case ChassisControlPowerCycle:
		return "Power cycle"
	case ChassisControlHardReset:
		return "Hard reset"
	case ChassisControlDiagnosticInterrupt:
		return "Diagnostic interrupt"
	case ChassisControlSoftPowerOff:
		return "Soft power off"
	default:
		return "Unknown"
	}
}

func (c ChassisControl) String() string {
	return fmt.Sprintf("%v(%v)", uint8(c), c.Description())
}

// ChassisControlReq represents a Chassis Control command, specified in section
// 22.3 and 28.3 of IPMI v1.5 and 2.0 respectively.
type ChassisControlReq struct {
	layers.BaseLayer

	// ChassisControl is the control command to send to the BMC, e.g. power up.
	ChassisControl ChassisControl
}

func (*ChassisControlReq) LayerType() gopacket.LayerType {
	return LayerTypeChassisControlReq
}

func (c *ChassisControlReq) SerializeTo(b gopacket.SerializeBuffer, _ gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(1)
	if err != nil {
		return err
	}
	bytes[0] = uint8(c.ChassisControl)
	return nil
}

type ChassisControlCmd struct {
	Req ChassisControlReq
}

// Name returns "Chassis Control".
func (*ChassisControlCmd) Name() string {
	return "Chassis Control"
}

// Operation returns OperationChassisControlReq.
func (*ChassisControlCmd) Operation() *Operation {
	return &OperationChassisControlReq
}

func (c *ChassisControlCmd) RemoteLUN() LUN {
	return LUNBMC
}

func (c *ChassisControlCmd) Request() gopacket.SerializableLayer {
	return &c.Req
}

func (c *ChassisControlCmd) Response() gopacket.DecodingLayer {
	return nil
}
