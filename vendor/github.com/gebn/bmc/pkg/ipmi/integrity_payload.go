package ipmi

import (
	"fmt"

	"github.com/google/gopacket"
)

// IntegrityPayload indicates a single integrity algorithm preference
// embedded in an RMCP+ Open Session Request message. One or more of these may
// be specified to indicate support for multiple algorithms, however this is
// uncommon (there is no mechanism in OpenIPMI for multiple payloads of a given
// type).
type IntegrityPayload struct {

	// Wildcard asks the BMC to choose an algorithm based on the requested max
	// privilege level. If this is true, Algorithm is null.
	Wildcard bool

	// Algorithm is the integrity algorithm to indicate support for. If
	// this is non-null, Wildcard is false.
	Algorithm IntegrityAlgorithm
}

// Serialise encodes the integrity payload onto the end of a buffer, returning
// an error if one occurs.
func (i *IntegrityPayload) Serialise(b gopacket.SerializeBuffer) error {
	d, err := b.AppendBytes(8)
	if err != nil {
		return err
	}
	d[0] = 0x01 // integrity payload
	d[1] = 0x00 // reserved
	d[2] = 0x00 // reserved
	if i.Wildcard {
		// wildcard is indicated by a 0-length packet, meaning all packets of a
		// given payload type must be the same length
		d[3] = 0x00
		d[4] = 0x00
	} else {
		d[3] = 0x08
		d[4] = uint8(i.Algorithm)
	}
	d[5] = 0x00
	d[6] = 0x00
	d[7] = 0x00
	return nil
}

// Deserialise reads an integrity payload from the supplied byte slice,
// returning unconsumed remaining bytes representing other payloads. If the
// payload is invalid, a nil slice is returned, and the payload is left in an
// unspecified state.
func (i *IntegrityPayload) Deserialise(d []byte, df gopacket.DecodeFeedback) ([]byte, error) {
	if len(d) < 8 {
		df.SetTruncated()
		return nil, fmt.Errorf("integrity payloads are 8 bytes, only %v remaining", len(d))
	}
	if d[0] != 0x01 {
		return nil, fmt.Errorf("data does not represent an integrity payload")
	}
	i.Wildcard = d[3] == 0x00
	i.Algorithm = IntegrityAlgorithm(d[4] & 0x3f)
	if i.Wildcard && i.Algorithm != IntegrityAlgorithmNone {
		return nil, fmt.Errorf("if integrity algorithm is wildcard, concrete algorithm must be None")
	}
	return d[8:], nil
}
