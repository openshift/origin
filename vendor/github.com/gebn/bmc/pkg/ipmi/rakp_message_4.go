package ipmi

import (
	"encoding/binary"
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// RAKPMessage4 is sent by the managed system in response to a RAKP Message 3,
// and is defined in section 13.23.
type RAKPMessage4 struct {
	layers.BaseLayer

	// Tag is equal to the tag sent by the remote console in RAKP Message 3.
	Tag uint8

	// Status indicates whether the remote console's RAKP Message 3 was
	// accepted, and if not, why not.
	Status StatusCode

	// RemoteConsoleSessionID can be used by the remote console along with the
	// tag to determine which session this response pertains to.
	RemoteConsoleSessionID uint32

	// if the status is non-zero, the packet is truncated here

	// ICV is an integrity check value over this message. Its size depends
	// on the algorithm; if we are using None, it will be empty. If we are using
	// MD5-128, it is not a signature.
	ICV []byte
}

func (*RAKPMessage4) LayerType() gopacket.LayerType {
	return LayerTypeRAKPMessage4
}

func (r *RAKPMessage4) CanDecode() gopacket.LayerClass {
	return r.LayerType()
}

func (*RAKPMessage4) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypePayload
}

func (r *RAKPMessage4) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 8 { // minimum in case of non-zero status code
		df.SetTruncated()
		return fmt.Errorf("RAKP Message 4 must be at least 8 bytes, got %v", len(data))
	}
	r.BaseLayer.Contents = data
	r.Tag = uint8(data[0])
	r.Status = StatusCode(data[1])
	// [2:4] reserved
	r.RemoteConsoleSessionID = binary.LittleEndian.Uint32(data[4:8])
	if r.Status == StatusCodeOK && len(data) > 8 {
		r.ICV = data[8:]
	} else {
		r.ICV = nil
	}
	return nil
}
