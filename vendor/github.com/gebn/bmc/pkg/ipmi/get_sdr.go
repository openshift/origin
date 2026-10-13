package ipmi

import (
	"encoding/binary"
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// GetSDRReq represents a request to retrieve a single Sensor Data Record from
// the BMC's SDR Repository Device. This command is specified in section 27.12
// and 33.12 of IPMI v1.5 and 2.0 respectively.
//
// No guarantees are made about the ordering of returned SDRs. Record IDs tend
// to be returned in ascending order, but have big gaps between numbers. The
// underlying entities and instances are scrambled - it cannot be assumed that
// when the next SDR has a different entity, there is no more of the current
// entity. The specification recommends retrieving all SDRs and incrementally
// indexing or processing them as needed.
type GetSDRReq struct {
	layers.BaseLayer

	// ReservationID is a consistency token, required if Offset > 0. If
	// provided, the request will fail if the SDR Repo device believes any
	// Record IDs that existed before the reservation was created may have
	// changed.
	ReservationID ReservationID

	// RecordID is the unique identifier of the SDR to read. To read the first
	// record, specify RecordIDFirst.
	RecordID RecordID

	// Offset is the number of bytes into the record to start reading from. If
	// >0, ReservationID must be non-zero.
	Offset uint8

	// Length is the number of bytes to read starting at the offset. Note that
	// 0xff is a sentinel value. It doesn't mean 255 bytes, it means the entire
	// record. Apparently this is more than most buffer sizes, so in "most
	// cases", will cause the BMC to return 0xca (or 0xff - it should be
	// interpreted the same in this case) as the completion code.
	Length uint8
}

func (*GetSDRReq) LayerType() gopacket.LayerType {
	return LayerTypeGetSDRReq
}

func (s *GetSDRReq) SerializeTo(b gopacket.SerializeBuffer, _ gopacket.SerializeOptions) error {
	bytes, err := b.PrependBytes(6)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint16(bytes[0:2], uint16(s.ReservationID))
	binary.LittleEndian.PutUint16(bytes[2:4], uint16(s.RecordID))
	bytes[4] = s.Offset
	bytes[5] = s.Length
	return nil
}

// GetSDRRsp contains the next Record ID in the SDR Repo, and wraps the SDR data
// requested.
type GetSDRRsp struct {
	layers.BaseLayer

	// Next is the Record ID of the "next" record in the SDR repository. If the
	// current record has RecordIDLast, and this is equal to RecordIDLast, the
	// end of the repository has been reached.
	Next RecordID
}

func (*GetSDRRsp) LayerType() gopacket.LayerType {
	return LayerTypeGetSDRRsp
}

func (s *GetSDRRsp) CanDecode() gopacket.LayerClass {
	return s.LayerType()
}

func (*GetSDRRsp) NextLayerType() gopacket.LayerType {
	return LayerTypeSDR
}

func (s *GetSDRRsp) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 2 {
		df.SetTruncated()
		return fmt.Errorf("response must be at least 2 bytes for the record ID, got %v",
			len(data))
	}

	s.BaseLayer.Contents = data[:2]
	s.BaseLayer.Payload = data[2:]
	s.Next = RecordID(binary.LittleEndian.Uint16(data[:2]))
	return nil
}

type GetSDRCmd struct {
	Req GetSDRReq
	Rsp GetSDRRsp
}

// Name returns "Get SDR".
func (*GetSDRCmd) Name() string {
	return "Get SDR"
}

// Operation returns &OperationGetSDRReq.
func (*GetSDRCmd) Operation() *Operation {
	return &OperationGetSDRReq
}

func (c *GetSDRCmd) RemoteLUN() LUN {
	return LUNBMC
}

func (c *GetSDRCmd) Request() gopacket.SerializableLayer {
	return &c.Req
}

func (c *GetSDRCmd) Response() gopacket.DecodingLayer {
	return &c.Rsp
}
