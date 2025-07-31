package ipmi

import (
	"crypto/hmac"
	"encoding/binary"
	"fmt"
	"hash"

	"github.com/gebn/bmc/pkg/iana"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// V2Session represents an IPMI v2.0/RMCP+ session header. Its format is
// specified in section 13.6 of the spec. N.B. the default instance of this
// struct can only deal with unauthenticated packets. The IntegrityAlgorithm
// attribute must be set to deal with all packets.
type V2Session struct {
	layers.BaseLayer
	PayloadDescriptor

	// Encrypted is true if the payload is encrypted, false if it is not.
	Encrypted bool

	// Authenticated is true if the payload is signed, false if it is not. Iff
	// this is false, the Pad and Authentication fields are absent from the wire
	// format.
	Authenticated bool

	// ID is the session ID, null for messages outside of a session. For
	// received packets, this is our session ID. For sent packets, this is the
	// other end's session ID. Note that these two numbers are separate, and
	// exchanged in the Open Session Request/Response messages.
	ID uint32

	// Sequence is the session sequence number, incremented regardless of
	// whether the packet is a retry. The sequence number is intended to be used
	// for rejecting replayed packets. Note RMCP+ sessions use a pair of
	// sequence numbers for authenticated packets, and another independent pair
	// for unauthenticated packets (4 in total). An endpoint verifies the
	// sequence number is expected before checking integrity (6.12.13), then
	// verifies integrity, then accepts the sequence number.
	Sequence uint32

	// Length is the size of the payload in bytes. This will never be 0.
	Length uint16

	// the payload data logically goes here

	// Pad is the number of 0xff bytes added after the payload data to make the
	// range over which the authentication data is created a multiple of 4
	// bytes. Valid values are therefore 0 through 3.
	Pad uint8

	// next header is a reserved field, always 0x07

	// Signature is the authentication data calculated according to the
	// integrity algorithm negotiated during session establishment.
	Signature []byte

	// IntegrityAlgorithm is an instance of the integrity algorithm negotiated
	// during session establishment. If this is an HMAC, it is already loaded
	// with the appropriate key. If this is nil, serialisation and
	// deserialisation of authenticated packets will fail.
	//
	// We assume all ICVs are calculated from the auth type field (0x6) to the
	// next header field (0x7) - which is the case for all algorithms in the
	// spec (13.28.4).
	IntegrityAlgorithm hash.Hash

	// ConfidentialityLayerType is the type of the layer that can decode
	// encrypted payloads. This value is returned when the payload is an IPMI
	// message and Encrypted is true. If this is nil, decoding of encrypted
	// packets will not work.
	ConfidentialityLayerType gopacket.LayerType
}

func (*V2Session) LayerType() gopacket.LayerType {
	return LayerTypeV2Session
}

func (s *V2Session) CanDecode() gopacket.LayerClass {
	return s.LayerType()
}

func (s *V2Session) NextLayerType() gopacket.LayerType {
	layerType := s.PayloadDescriptor.NextLayerType()
	if layerType == LayerTypeMessage && s.Encrypted {
		// special case - this must be handled here, because lower layers don't
		// know whether it's encrypted. I imagine the spec authors left the
		// encrypted bit in the session layer, so it could apply to OEM payloads,
		// but in practice this just leads to hacks like this. The correct way
		// is for there to be an IPMI message payload layer, containing just
		// this boolean and an encrypted or unencrypted IPMI message.
		return s.ConfidentialityLayerType
	}
	return layerType
}

// executeHash is a convenience function to calculate the hash of a slice of
// bytes. It leaves the hash in a reset state. Note that this function cannot be
// called concurrently on a single underlying hash.Hash.
func executeHash(h hash.Hash, b []byte) []byte {
	if h == nil {
		return nil
	}
	h.Write(b)
	sum := h.Sum(nil)
	h.Reset()
	return sum
}

func (s *V2Session) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	// assuming not OEM payload type, not authenticated, and 0-length payload
	if len(data) < 12 {
		df.SetTruncated()
		return fmt.Errorf("session packet too small for header fields: want 12 bytes, got %v", len(data))
	}

	if AuthenticationType(data[0]) != AuthenticationTypeRMCPPlus {
		return fmt.Errorf("the first byte of an RMCP+ session header is always 0x6 (auth type RMCP+), otherwise it is a v1.5 session wrapper")
	}
	s.Encrypted = data[1]&(1<<7) != 0
	s.Authenticated = data[1]&(1<<6) != 0
	s.PayloadType = PayloadType(data[1] & 0x3f) // lower 6 bits

	// as the OEM fields may or may not exist, we have to base future indices
	// into the data slice on a variable
	offset := 2
	if s.PayloadType == PayloadTypeOEM {
		// OEM fields are 6 bytes, so packet must now be at least 12 + 6 bytes long
		if len(data) < 18 {
			df.SetTruncated()
			return fmt.Errorf("session packet too small for OEM fields; want 18 bytes, got %v", len(data))
		}
		s.Enterprise = iana.Enterprise(binary.LittleEndian.Uint32(data[2:6]))
		s.PayloadID = binary.LittleEndian.Uint16(data[6:8])
		offset += 6 // offset all future indices by another 6 to account for OEM fields
	} else {
		s.Enterprise = 0
		s.PayloadID = 0
	}
	s.ID = binary.LittleEndian.Uint32(data[offset : offset+4])
	s.Sequence = binary.LittleEndian.Uint32(data[offset+4 : offset+8])
	s.Length = binary.LittleEndian.Uint16(data[offset+8 : offset+10])
	offset += 10

	s.BaseLayer.Contents = data[0:offset] // we ignore the trailer, which may not even be there
	if len(data) < offset+int(s.Length) {
		df.SetTruncated()
		return fmt.Errorf("session packet shorter than payload length field suggests: want %v bytes, got %v", offset+int(s.Length), len(data))
	}
	s.BaseLayer.Payload = data[offset : offset+int(s.Length)]

	offset += int(s.Length)

	// there may not be any bytes left; we haven't done any length validation
	// beyond this point
	if !s.Authenticated {
		// assume len(data) == offset; if not, ignore additional bytes
		s.Pad = 0
		s.Signature = nil
		return nil
	}

	// consume pad bytes until we reach the end of the packet
	padStart := offset
	for b := uint8(0xFF); offset < len(data) && b == 0xFF; offset++ {
		b = data[offset]
	}
	offset-- // -1 because offset was incremented once for the first byte that was not 0xff
	s.Pad = uint8(offset - padStart)

	// ignore next two bytes: pad length (which we've calculated) and next header
	offset += 2

	if len(data) < offset {
		s.Signature = nil
		df.SetTruncated()
		return fmt.Errorf("session packet too short for auth code; want %v bytes, got %v", offset, len(data))
	}

	s.Signature = data[offset:]
	signature := executeHash(s.IntegrityAlgorithm, data[:offset])
	if !hmac.Equal(s.Signature, signature) {
		return fmt.Errorf("invalid signature: want %v, got %v", signature, s.Signature)
	}

	return nil
}

func (s *V2Session) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	headerLength := 12
	if s.PayloadType == PayloadTypeOEM {
		headerLength += 6
	}

	if opts.FixLengths {
		s.Length = uint16(len(b.Bytes()))

		// pad is only relevant when the trailer is present, i.e. when the
		// packet is authenticated
		if s.Authenticated {
			authCodeRangeLength := headerLength + int(s.Length) + 2
			s.Pad = uint8((4 - authCodeRangeLength%4) % 4) // {0, 1, 2, 3}
		}
	}

	header, err := b.PrependBytes(headerLength)
	if err != nil {
		return err
	}

	// fill header
	header[0] = uint8(AuthenticationTypeRMCPPlus)
	header[1] = uint8(s.PayloadType)
	if s.Encrypted {
		header[1] |= 1 << 7
	}
	if s.Authenticated {
		header[1] |= 1 << 6
	}

	offset := 2
	if s.PayloadType == PayloadTypeOEM {
		binary.LittleEndian.PutUint32(header[2:6], uint32(s.Enterprise))
		binary.LittleEndian.PutUint16(header[6:8], uint16(s.PayloadID))
		offset += 6
	}

	binary.LittleEndian.PutUint32(header[offset:offset+4], s.ID)
	binary.LittleEndian.PutUint32(header[offset+4:offset+8], s.Sequence)
	binary.LittleEndian.PutUint16(header[offset+8:offset+10], s.Length)

	if s.Authenticated {
		// we could optimise this down to a single append if
		// !opts.ComputeChecksums, but we assume all packets are authenticated

		// append trailer excluding signature
		trailerExSignatureLength := int(s.Pad) + 2
		trailerExSignature, err := b.AppendBytes(trailerExSignatureLength)
		if err != nil {
			return err
		}

		// write trailer
		for i := 0; i < int(s.Pad); i++ {
			trailerExSignature[i] = 0xff
		}
		offset = int(s.Pad)
		trailerExSignature[offset] = s.Pad
		trailerExSignature[offset+1] = 0x07

		// if requested, calculate signature
		if opts.ComputeChecksums {
			s.Signature = executeHash(s.IntegrityAlgorithm, b.Bytes())
		}

		// append signature
		trailerSignature, err := b.AppendBytes(len(s.Signature))
		if err != nil {
			return err
		}

		// write signature
		copy(trailerSignature, s.Signature)
	}
	return nil
}
