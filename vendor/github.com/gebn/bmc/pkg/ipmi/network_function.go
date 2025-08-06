package ipmi

import (
	"fmt"
)

// NetworkFunction is a network function code, or "NetFn". This identifies the
// functional class of a message, e.g. chassis device, sensor readings, firmware
// transfer etc. It can be thought of as the category for the command. For IPMB,
// each class has an adjacent pair of function codes, where the lower (even)
// number is used for requests and the upper (odd) number is used for responses
// to those requests. This is a 6-bit uint on the wire. See section 5.1 of the
// v1.5 or v2.0 spec for definitions.
//
// For example, Get System GUID's network function is "App", which corresponds
// to 0x6 and 0x7. If sending a request to the BMC, we would set our NetFn to
// 0x6.
type NetworkFunction uint8

const (
	NetworkFunctionChassisReq NetworkFunction = 0x0
	NetworkFunctionChassisRsp NetworkFunction = 0x1

	NetworkFunctionBridgeReq NetworkFunction = 0x2
	NetworkFunctionBridgeRsp NetworkFunction = 0x3

	NetworkFunctionSensorReq NetworkFunction = 0x4
	NetworkFunctionSensorRsp NetworkFunction = 0x5

	NetworkFunctionAppReq NetworkFunction = 0x6
	NetworkFunctionAppRsp NetworkFunction = 0x7

	NetworkFunctionFirmwareReq NetworkFunction = 0x8
	NetworkFunctionFirmwareRsp NetworkFunction = 0x9

	NetworkFunctionStorageReq NetworkFunction = 0xa
	NetworkFunctionStorageRsp NetworkFunction = 0xb

	NetworkFunctionTransportReq NetworkFunction = 0xc
	NetworkFunctionTransportRsp NetworkFunction = 0xd

	NetworkFunctionGroupReq NetworkFunction = 0x2c
	NetworkFunctionGroupRsp NetworkFunction = 0x2d

	NetworkFunctionOEMReq NetworkFunction = 0x2e
	NetworkFunctionOEMRsp NetworkFunction = 0x2f
)

// IsRequest indicates whether the given network function code is used for
// request or response messages.
func (n NetworkFunction) IsRequest() bool {
	return uint8(n)%2 == 0
}

func (n NetworkFunction) name() string {
	switch n {
	case NetworkFunctionChassisReq, NetworkFunctionChassisRsp:
		return "Chassis"
	case NetworkFunctionBridgeReq, NetworkFunctionBridgeRsp:
		return "Bridge"
	case NetworkFunctionSensorReq, NetworkFunctionSensorRsp:
		return "Sensor/Event"
	case NetworkFunctionAppReq, NetworkFunctionAppRsp:
		return "App"
	case NetworkFunctionFirmwareReq, NetworkFunctionFirmwareRsp:
		return "Firmware"
	case NetworkFunctionStorageReq, NetworkFunctionStorageRsp:
		return "Storage"
	case NetworkFunctionTransportReq, NetworkFunctionTransportRsp:
		return "Transport"
	case NetworkFunctionGroupReq, NetworkFunctionGroupRsp:
		return "Group Extension"
	case NetworkFunctionOEMReq, NetworkFunctionOEMRsp:
		return "OEM/Group"
	}
	if n >= 0xe && n <= 0x2b {
		return "Reserved"
	}
	if n >= 0x30 && n <= 0x3f {
		return "Controller-specific OEM/Group"
	}
	return "Unknown"
}

func (n NetworkFunction) String() string {
	variety := "Request"
	if !n.IsRequest() {
		variety = "Response"
	}
	return fmt.Sprintf("%#x(%v %v)", uint8(n), n.name(), variety)
}
