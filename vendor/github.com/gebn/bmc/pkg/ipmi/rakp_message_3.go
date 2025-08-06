package ipmi

import (
	"encoding/binary"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// RAKPMessage3 is sent by the remote console in response to a RAKP Message 2,
// and is defined in section 13.22.
type RAKPMessage3 struct {
	layers.BaseLayer

	// Tag is an arbitrary 8-bit quantity used by the remote console to match
	// this message with RAKP Message 4.
	Tag uint8

	// Status indicates whether the managed system's RAKP Message 2 was
	// accepted. If it was not, the BMC will remove the session state, and the
	// remote console will have to start again from RAKP Message 1.
	Status StatusCode

	// ManagedSystemSessionID is the session ID returned by the BMC in the RMCP+
	// Open Session Response message.
	ManagedSystemSessionID uint32

	// if the status is non-zero, the packet is truncated here

	// AuthCode is an integrity check value over this message. Its size depends
	// on the algorithm; if we are using None, it will be empty.
	AuthCode []byte
}

func (*RAKPMessage3) LayerType() gopacket.LayerType {
	return LayerTypeRAKPMessage3
}

func (r *RAKPMessage3) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	length := 8
	if r.Status == StatusCodeOK {
		length += len(r.AuthCode)
	}
	d, err := b.PrependBytes(length)
	if err != nil {
		return err
	}
	d[0] = r.Tag
	d[1] = uint8(r.Status)
	d[2] = 0x00
	d[3] = 0x00
	binary.LittleEndian.PutUint32(d[4:8], r.ManagedSystemSessionID)
	if r.Status == StatusCodeOK {
		copy(d[8:], r.AuthCode)
	} else {
		r.AuthCode = nil
	}
	return nil
}

type RAKPMessage3Payload struct {
	Req RAKPMessage3
	Rsp RAKPMessage4
}

// Descriptor returns PayloadDescriptorRAKPMessage3.
func (*RAKPMessage3Payload) Descriptor() *PayloadDescriptor {
	return &PayloadDescriptorRAKPMessage3
}

func (p *RAKPMessage3Payload) Request() gopacket.SerializableLayer {
	return &p.Req
}

func (p *RAKPMessage3Payload) Response() gopacket.DecodingLayer {
	return &p.Rsp
}
