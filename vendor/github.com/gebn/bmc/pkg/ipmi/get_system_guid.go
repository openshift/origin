package ipmi

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// GetSystemGUIDRsp is the response to a Get System GUID command, specified in
// 18.13 and 22.14 of IPMI v1.5 and 2.0 respectively.
type GetSystemGUIDRsp struct {
	layers.BaseLayer

	// GUID contains the BMC's globally unique ID in the original byte order.
	// This can be in any format, and any byte order, so cannot be interpreted
	// reliably without additional knowledge. As an approximation, treating the
	// original bytes as a GUID seems to work fairly well; on Dell this matches
	// the smbiosGUID field in the iDRAC UI, and on Quanta it produces a valid
	// version 1 GUID.
	GUID [16]byte
}

func (*GetSystemGUIDRsp) LayerType() gopacket.LayerType {
	return LayerTypeGetSystemGUIDRsp
}

func (g *GetSystemGUIDRsp) CanDecode() gopacket.LayerClass {
	return g.LayerType()
}

func (*GetSystemGUIDRsp) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (g *GetSystemGUIDRsp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 16 {
		df.SetTruncated()
		return fmt.Errorf("GUID must be 16 bytes long, got %v", len(data))
	}

	g.BaseLayer.Contents = data[:16]
	copy(g.GUID[:], data[:16])
	return nil
}

type GetSystemGUIDCmd struct {
	Rsp GetSystemGUIDRsp
}

// Name returns "Get System GUID".
func (*GetSystemGUIDCmd) Name() string {
	return "Get System GUID"
}

// Operation returns OperationGetSystemGUIDReq.
func (*GetSystemGUIDCmd) Operation() *Operation {
	return &OperationGetSystemGUIDReq
}

func (*GetSystemGUIDCmd) RemoteLUN() LUN {
	return LUNBMC
}

func (*GetSystemGUIDCmd) Request() gopacket.SerializableLayer {
	return nil
}

func (c *GetSystemGUIDCmd) Response() gopacket.DecodingLayer {
	return &c.Rsp
}
