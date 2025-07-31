package layerexts

import (
	"github.com/google/gopacket"
)

// SerializableDecodingLayer is satisfied by layers that can be both serialised
// and decoded. It is intended to be used for layers that can be both sent and
// received.
type SerializableDecodingLayer interface {
	gopacket.SerializableLayer
	gopacket.DecodingLayer
}
