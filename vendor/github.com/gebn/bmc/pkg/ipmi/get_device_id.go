package ipmi

import (
	"encoding/binary"
	"fmt"

	"github.com/gebn/bmc/internal/pkg/bcd"
	"github.com/gebn/bmc/pkg/iana"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// GetDeviceIDRsp represents the response to a Get Device ID command, specified
// in 17.1 of IPMI v1.5 and 20.1 of IPMI v2.0.
type GetDeviceIDRsp struct {
	layers.BaseLayer

	// ID is the device ID. 0x00 means unspecified.
	ID uint8

	// ProvidesSDRs indicates whether the device provides Device SDRs.
	ProvidesSDRs bool

	// Revision contains the device revision. This is a 4-bit uint on the wire.
	Revision uint8

	// Available is true if the device is under normal operation - that is, not
	// performing an SDR repository update, firmware update or
	// self-initialisation. The first two can be distinguished by issuing Get
	// SDR and looking at the completion code.
	Available bool

	// MajorFirmwareRevision is a 6-bit uint on the wire.
	MajorFirmwareRevision uint8

	MinorFirmwareRevision uint8

	// MajorIPMIVersion is the integral component of the IPMI specification
	// implemented by the BMC. This is not influenced by whether the Get Device
	// ID command was executed using a v1.5 session wrapper.
	MajorIPMIVersion uint8
	MinorIPMIVersion uint8

	SupportsChassisDevice            bool
	SupportsBridgeDevice             bool
	SupportsIPMBEventGeneratorDevice bool
	SupportsIPMBEventReceiverDevice  bool
	SupportsFRUInventoryDevice       bool
	SupportsSELDevice                bool
	SupportsSDRRepositoryDevice      bool
	SupportsSensorDevice             bool

	// Manufacturer is sometimes that of the BMC (Intel) or that of the
	// motherboard (SuperMicro, which uses an Aten BMC...). It is a 20-bit uint
	// on the wire; the most significant 4 bits are reserved, set to 0000b.
	// 0x000000 means unspecified.
	Manufacturer iana.Enterprise

	// Product is chosen by the manufacturer to identify a particular system,
	// module, add-in card or board set. 0x0000 means unspecified. This is not
	// reliable; it has observed to be the same on different Quanta models.
	Product uint16

	AuxiliaryFirmwareRevision [4]byte
}

func (*GetDeviceIDRsp) LayerType() gopacket.LayerType {
	return LayerTypeGetDeviceIDRsp
}

func (g *GetDeviceIDRsp) CanDecode() gopacket.LayerClass {
	return g.LayerType()
}

func (*GetDeviceIDRsp) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (g *GetDeviceIDRsp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 11 {
		df.SetTruncated()
		return fmt.Errorf("Get Device ID response must be at least 11 bytes excluding completion code; got %v", len(data))
	}

	g.BaseLayer.Contents = data
	g.ID = uint8(data[0])
	g.ProvidesSDRs = data[1]&(1<<7) != 0
	g.Revision = uint8(data[1] & 0x0f)
	g.Available = data[2]&(1<<7) == 0
	g.MajorFirmwareRevision = uint8(data[2] & 0x7f)
	g.MinorFirmwareRevision = bcd.Decode(data[3])
	g.MajorIPMIVersion = uint8(data[4] & 0xf)
	g.MinorIPMIVersion = uint8(data[4] >> 4)
	g.SupportsChassisDevice = data[5]&(1<<7) != 0
	g.SupportsBridgeDevice = data[5]&(1<<6) != 0
	g.SupportsIPMBEventGeneratorDevice = data[5]&(1<<5) != 0
	g.SupportsIPMBEventReceiverDevice = data[5]&(1<<4) != 0
	g.SupportsFRUInventoryDevice = data[5]&(1<<3) != 0
	g.SupportsSELDevice = data[5]&(1<<2) != 0
	g.SupportsSDRRepositoryDevice = data[5]&(1<<1) != 0
	g.SupportsSensorDevice = data[5]&1 != 0
	g.Manufacturer = iana.Enterprise(uint32(data[6]) | uint32(data[7])<<8 |
		uint32(data[8])<<16)
	g.Product = binary.LittleEndian.Uint16(data[9:11])
	if len(data) > 11 {
		copy(g.AuxiliaryFirmwareRevision[:], data[11:])
	} else {
		g.AuxiliaryFirmwareRevision = [4]byte{}
	}
	return nil
}

type GetDeviceIDCmd struct {
	Rsp GetDeviceIDRsp
}

// Name returns "Get Device ID".
func (*GetDeviceIDCmd) Name() string {
	return "Get Device ID"
}

// Operation returns OperationGetDeviceIDReq.
func (*GetDeviceIDCmd) Operation() *Operation {
	return &OperationGetDeviceIDReq
}

func (c *GetDeviceIDCmd) RemoteLUN() LUN {
	return LUNBMC
}

func (c *GetDeviceIDCmd) Request() gopacket.SerializableLayer {
	return nil
}

func (c *GetDeviceIDCmd) Response() gopacket.DecodingLayer {
	return &c.Rsp
}
