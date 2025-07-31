package ipmi

import (
	"encoding/binary"
	"fmt"

	"github.com/gebn/bmc/internal/pkg/bcd"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// SDR represents a Sensor Data Record header, outlined at the beginning of 37
// and 43 of IPMI v1.5 and 2.0 respectively. These fields are common to all
// SDRs, and is the limit of what the SDR Repository Device cares about: the
// record key and body are opaque bytes.
//
// Despite the name, an SDR may not pertain to a sensor, e.g. there are Device
// Locator and Entity Association SDR types.
type SDR struct {
	layers.BaseLayer

	// ID is the current Record ID for the SDR. This is not the record key (that
	// is a set of fields specific to the record type), and may change if the
	// SDR Repository is modified. See RecordID documentation for more details.
	ID RecordID

	// Version is the version number of the SDR specification. It is used with
	// the Type field to control how the record is parsed. We return an error
	// during decoding if this is not supported.
	Version uint8

	// Type indicates what the SDR describes. Confusingly, not all SDRs pertain
	// to sensors.
	Type RecordType

	// Length is the number of remaining bytes in the payload (i.e. after the header).
	//
	// This means the max SDR size on the wire is 260 bytes. In practice, OEM
	// records notwithstanding, it is unlikely to be >60.
	//
	// If it weren't for this field, the limit for the whole SDR including
	// header could theoretically be 255 + the max supported payload size (the
	// SDR Repo Device commands provide no way to address subsequent sections
	// for reading).
	Length uint8

	// payload contains the record key and body.
}

func (*SDR) LayerType() gopacket.LayerType {
	return LayerTypeSDR
}

func (s *SDR) CanDecode() gopacket.LayerClass {
	return s.LayerType()
}

func (s *SDR) NextLayerType() gopacket.LayerType {
	// there may eventually be a need to switch on both Type and Version
	return s.Type.NextLayerType()
}

func (s *SDR) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 5 {
		df.SetTruncated()
		return fmt.Errorf("SDR Header is always 5 bytes, got %v", len(data))
	}
	s.ID = RecordID(binary.LittleEndian.Uint16(data[0:2]))
	s.Version = bcd.Decode(data[2]&0xf)*10 + bcd.Decode(data[2]>>4)
	s.Type = RecordType(data[3])
	s.Length = uint8(data[4])

	s.BaseLayer.Contents = data[:5]
	s.BaseLayer.Payload = data[5:]
	return nil
}
