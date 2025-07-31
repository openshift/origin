package ipmi

import (
	"encoding/binary"
	"fmt"
	"hash"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// V1Session represents the IPMI v1.5 session header. It wraps all IPMI commands.
// The zero value is suitable for commands sent "outside" of a session, e.g. Get
// Channel Authentication Capabilities, and Get Device GUID. See 6.11.7 for more
// details.
type V1Session struct {
	layers.BaseLayer

	// AuthType indicates the algorithm protecting the inner message, whose
	// signature is contained in the AuthCode field. If this is
	// AuthenticationTypeNone, the AuthCode field will be skipped when
	// serialising. As this struct is for IPMI v1.x only,
	// AuthenticationTypeRMCPPlus is an invalid value.
	AuthType AuthenticationType

	// Sequence is the session sequence number, intended to protect against
	// replay attacks. A given session has inbound and outbound sequence
	// numbers, so the one this corresponds to will depend on whether we're
	// sending or receiving a packet. Each end selects the starting sequence
	// number for the messages they receive. The sequence number increments for
	// retransmits.
	//
	// Note this is not used for matching responses with requests; that is done
	// one level further down, with the IPMI message sequence field.
	Sequence uint32

	// ID is the session ID, chosen by the BMC and sent in the Activate Session
	// response. This will be a temporary ID during session initialisation.
	ID uint32

	// AuthCode is a signature whose format depends on the AuthType. This field
	// is absent from the wire format if AuthType == AuthTypeNone. Whether this
	// is present or not when the Authentication Type = OEM is dependent on the
	// OEM identified in the Get Channel Authentication Capabilities command.
	AuthCode [16]byte

	// Length is the length of the contained IPMI message.
	Length uint8

	// AuthenticationAlgorithm is called to generate a checksum of the packet.
	AuthenticationAlgorithm hash.Hash
}

func (*V1Session) LayerType() gopacket.LayerType {
	return LayerTypeV1Session
}

func (s *V1Session) CanDecode() gopacket.LayerClass {
	return s.LayerType()
}

func (s *V1Session) NextLayerType() gopacket.LayerType {
	return LayerTypeMessage
}

func (s *V1Session) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 10 { // 10 is the min length without an auth code
		df.SetTruncated()
		return fmt.Errorf("v1.5 session is at least 10 bytes, got %v bytes", len(data))
	}

	s.AuthType = AuthenticationType(data[0])
	s.Sequence = binary.LittleEndian.Uint32(data[1:5])
	s.ID = binary.LittleEndian.Uint32(data[5:9])
	if s.AuthType == AuthenticationTypeNone {
		// not expecting an auth code
		s.BaseLayer.Contents = data[:10]
		s.BaseLayer.Payload = data[10:]
		s.Length = uint8(data[9])
	} else {
		// there should be an auth code
		if len(data) < 26 {
			df.SetTruncated()
			return fmt.Errorf("v1.5 session is 26 bytes with an auth code, got %v bytes", len(data))
		}

		s.BaseLayer.Contents = data[:26]
		s.BaseLayer.Payload = data[26:]
		copy(s.AuthCode[:], data[9:25]) // TODO work out byte order - probably need to reverse
		s.Length = uint8(data[25])
	}
	return nil
}

func (s *V1Session) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	if opts.FixLengths {
		// length is the number of bytes left in the payload after this header
		s.Length = uint8(len(b.Bytes()))
	}

	size := 10
	if s.AuthType != AuthenticationTypeNone {
		size += 16 // for AuthCode
	}

	bytes, err := b.PrependBytes(size)
	if err != nil {
		return err
	}

	bytes[0] = uint8(s.AuthType)
	binary.LittleEndian.PutUint32(bytes[1:5], s.Sequence)
	binary.LittleEndian.PutUint32(bytes[5:9], s.ID)
	if s.AuthType == AuthenticationTypeNone {
		bytes[9] = s.Length
	} else {
		copy(bytes[9:25], s.AuthCode[:]) // TODO work out byte order - probably need to reverse
		bytes[25] = s.Length
	}
	return nil
}
