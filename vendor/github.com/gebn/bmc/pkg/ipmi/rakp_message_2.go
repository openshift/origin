package ipmi

import (
	"encoding/binary"
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// RAKPMessage2 is sent by the managed system in response to a RAKP Message 1,
// and is defined in section 13.21.
type RAKPMessage2 struct {
	layers.BaseLayer

	// Tag is equal to the tag sent by the remote console in RAKP Message 1.
	Tag uint8

	// Status indicates whether the remote console's RAKP Message 1 was
	// accepted, and if not, why not.
	Status StatusCode

	// RemoteConsoleSessionID can be used by the remote console along with the
	// tag to determine which session this response pertains to.
	RemoteConsoleSessionID uint32

	// if the status is non-zero, the packet is truncated here

	// ManagedSystemRandom is a random 16-byte value selected by the managed
	// system. Although it is referred to as a number in the spec, its byte
	// order is not reversed on the wire.
	ManagedSystemRandom [16]byte

	// ManagedSystemGUID is typically specified by the BMC's SMBIOS
	// implementation. It is opaque for our purposes. The spec suggests the
	// remote console is meant to validate this is as expected, however this
	// requires an inventory. A test Supermicro X10 BMC returns all 0s for this
	// field.
	ManagedSystemGUID [16]byte

	// AuthCode is an integrity check value over this message. Its size depends
	// on the algorithm; if we are using None, it will be empty. If we are using
	// MD5-128, it is not a signature.
	AuthCode []byte
}

func (*RAKPMessage2) LayerType() gopacket.LayerType {
	return LayerTypeRAKPMessage2
}

func (r *RAKPMessage2) CanDecode() gopacket.LayerClass {
	return r.LayerType()
}

func (*RAKPMessage2) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (r *RAKPMessage2) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 8 { // minimum in case of non-zero status code
		df.SetTruncated()
		return fmt.Errorf("RAKP Message 2 must be at least 8 bytes, got %v", len(data))
	}
	r.BaseLayer.Contents = data
	r.Tag = uint8(data[0])
	r.Status = StatusCode(data[1])
	// [2:4] reserved
	r.RemoteConsoleSessionID = binary.LittleEndian.Uint32(data[4:8])
	if r.Status == StatusCodeOK {
		copy(r.ManagedSystemRandom[:], data[8:24])
		copy(r.ManagedSystemGUID[:], data[24:40])
		if len(data) > 40 {
			r.AuthCode = data[40:]
		} else {
			r.AuthCode = nil
		}
	} else {
		r.ManagedSystemRandom = [16]byte{}
		r.ManagedSystemGUID = [16]byte{}
		r.AuthCode = nil
	}
	return nil
}
