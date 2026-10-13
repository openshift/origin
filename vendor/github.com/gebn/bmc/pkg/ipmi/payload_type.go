package ipmi

import (
	"fmt"
)

// PayloadType identifies the layer immediately within the RMCP+ session
// wrapper. Values are specified in 13.27.3 of the IPMI v2.0 spec. This is a
// 6-bit uint on the wire.
type PayloadType uint8

const (
	// "standard" payload types

	PayloadTypeIPMI PayloadType = 0x0

	// PayloadTypeOEM means "check the OEM IANA and OEM payload ID to find out
	// what this actually is".
	PayloadTypeOEM PayloadType = 0x2

	// "session setup" payload types

	PayloadTypeOpenSessionReq PayloadType = 0x10
	PayloadTypeOpenSessionRsp PayloadType = 0x11
	PayloadTypeRAKPMessage1   PayloadType = 0x12
	PayloadTypeRAKPMessage2   PayloadType = 0x13
	PayloadTypeRAKPMessage3   PayloadType = 0x14
	PayloadTypeRAKPMessage4   PayloadType = 0x15
)

var (
	payloadTypeDescriptions = map[PayloadType]string{
		PayloadTypeIPMI:           "IPMI",
		PayloadTypeOEM:            "OEM Explicit",
		PayloadTypeOpenSessionReq: "RMCP+ Open Session Request",
		PayloadTypeOpenSessionRsp: "RMCP+ Open Session Response",
		PayloadTypeRAKPMessage1:   "RAKP Message 1",
		PayloadTypeRAKPMessage2:   "RAKP Message 2",
		PayloadTypeRAKPMessage3:   "RAKP Message 3",
		PayloadTypeRAKPMessage4:   "RAKP Message 4",
	}
)

func (p PayloadType) Description() string {
	if desc, ok := payloadTypeDescriptions[p]; ok {
		return desc
	}
	if p >= 0x20 && p <= 0x27 {
		return fmt.Sprintf("OEM%d", p-0x20)
	}
	if p > 0x3f {
		return "Invalid"
	}
	return "Unknown"
}

func (p PayloadType) String() string {
	return fmt.Sprintf("%#x(%v)", uint8(p), p.Description())
}
