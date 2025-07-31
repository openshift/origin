package ipmi

import (
	"encoding/binary"
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// ReserveSDRRepositoryRsp represents the response to a Reserve SDR Repository
// command, specified in 33.11 of IPMI v2.0.
type ReserveSDRRepositoryRsp struct {
	layers.BaseLayer

	ReservationID ReservationID
}

func (*ReserveSDRRepositoryRsp) LayerType() gopacket.LayerType {
	return LayerTypeReserveSDRRepositoryRsp
}

func (r *ReserveSDRRepositoryRsp) CanDecode() gopacket.LayerClass {
	return r.LayerType()
}

func (*ReserveSDRRepositoryRsp) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (r *ReserveSDRRepositoryRsp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 2 {
		df.SetTruncated()
		return fmt.Errorf("response must be at least 2 bytes, got %v", len(data))
	}

	r.BaseLayer.Contents = data[:2]
	r.ReservationID = ReservationID(binary.LittleEndian.Uint16(data[0:2]))
	return nil
}

type ReserveSDRRepositoryCmd struct {
	Rsp ReserveSDRRepositoryRsp
}

// Name returns "Reserve SDR Repository".
func (*ReserveSDRRepositoryCmd) Name() string {
	return "Reserve SDR Repository"
}

// Operation returns &OperationReserveSDRRepositoryReq.
func (*ReserveSDRRepositoryCmd) Operation() *Operation {
	return &OperationReserveSDRRepositoryReq
}

func (c *ReserveSDRRepositoryCmd) RemoteLUN() LUN {
	return LUNBMC
}

func (c *ReserveSDRRepositoryCmd) Request() gopacket.SerializableLayer {
	return nil
}

func (c *ReserveSDRRepositoryCmd) Response() gopacket.DecodingLayer {
	return &c.Rsp
}
