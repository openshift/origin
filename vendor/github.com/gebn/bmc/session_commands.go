package bmc

import (
	"context"

	"github.com/gebn/bmc/pkg/ipmi"
)

// SessionCommands contains high-level wrappers for sending commands within an
// established session. These commands are common to all versions of IPMI.
type SessionCommands interface {

	// SessionlessCommands enables all session-less commands to also be sent
	// inside a session; indeed it is convention for Get Channel Authentication
	// Capabilities to be used as a keepalive.
	SessionlessCommands

	// GetSessionInfo sends a Get Session Info command to the BMC. This is
	// specified in 18.18 and 22.20 of IPMI v1.5 and v2.0 respectively.
	GetSessionInfo(context.Context, *ipmi.GetSessionInfoReq) (*ipmi.GetSessionInfoRsp, error)

	// GetDeviceID sends a Get Device ID command to the BMC. This is specified
	// in 17.1 and 20.1 of IPMI v1.5 and 2.0 respectively.
	GetDeviceID(context.Context) (*ipmi.GetDeviceIDRsp, error)

	// GetChassisStatus sends a Get Chassis Status command to the BMC. This is
	// specified in 22.2 and 28.2 of IPMI v1.5 and 2.0 respectively.
	GetChassisStatus(context.Context) (*ipmi.GetChassisStatusRsp, error)

	// ChassisControl provides power up, power down and reset control. It is
	// specified in 22.3 and 28.3 of IPMI v1.5 and 2.0 respectively.
	ChassisControl(context.Context, ipmi.ChassisControl) error

	// GetSDRRepositoryInfo obtains information about the BMC's Sensor Data
	// Record Repository. It is specified in 27.9 and 33.9 of IPMI v1.5 and 2.0
	// respectively.
	GetSDRRepositoryInfo(context.Context) (*ipmi.GetSDRRepositoryInfoRsp, error)

	// ReserveSDRRepository sets the requester as the present "owner" of the
	// repository. The returned reservation ID must be included in requests that
	// either delete or partially read/write an SDR.
	// This is specified in 33.11 of IPMI v2.0.
	ReserveSDRRepository(context.Context) (*ipmi.ReserveSDRRepositoryRsp, error)

	// GetSensorReading retrieves the current value of a sensor, identified by
	// its number. It is specified in 29.14 and 35.14 of IPMI v1.5 and 2.0
	// respectively. Note, the raw value is in one of three formats, and is
	// converted into a "real" reading via one or more formulae - interpreting
	// it requires the SDR.
	GetSensorReading(context.Context, uint8) (*ipmi.GetSensorReadingRsp, error)

	// GetSessionPrivilegeLevel retrieves the current session privilege level. This is
	// specified in 18.16 and 22.18 of IPMI v1.5 and 2.0 respectively.
	GetSessionPrivilegeLevel(context.Context) (ipmi.PrivilegeLevel, error)

	// SetSessionPrivilegeLevel sends a Set Session Privilege Level command to the BMC. This is
	// specified in 18.16 and 22.18 of IPMI v1.5 and 2.0 respectively.
	// PrivilegeLevelHighest and PrivilegeLevelCallback are invalid values.
	SetSessionPrivilegeLevel(context.Context, ipmi.PrivilegeLevel) (ipmi.PrivilegeLevel, error)

	// closeSession sends a Close Session command to the BMC. It is unexported
	// as calling it randomly would leave the session in an invalid state. Call
	// Close() on the session itself to invoke this.
	closeSession(context.Context) error
}
