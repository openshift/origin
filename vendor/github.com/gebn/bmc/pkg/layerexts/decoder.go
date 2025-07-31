package layerexts

import (
	"github.com/google/gopacket"
)

// LayerDecodingLayer is satisfied by types that we can generate a decoder. This
// is lifted from gopacket, where it is not exported.
type LayerDecodingLayer interface {
	gopacket.Layer
	DecodeFromBytes([]byte, gopacket.DecodeFeedback) error
	NextLayerType() gopacket.LayerType
}

// BuildDecoder creates a gopacket.Decoder for a layer implementing the required
// methods. It is useful when creating a gopacket.LayerTypeMetadata, however
// note this decoder is not used in the context of gopacket.DecodingLayer. This
// function takes a generating function rather than concrete instance, as
// otherwise decoded layers will appear to overwrite each other.
func BuildDecoder(newLayer func() LayerDecodingLayer) gopacket.Decoder {
	return gopacket.DecodeFunc(func(d []byte, p gopacket.PacketBuilder) error {
		layer := newLayer()
		err := layer.DecodeFromBytes(d, p)
		if err != nil {
			return err
		}
		p.AddLayer(layer)
		next := layer.NextLayerType()
		if next == gopacket.LayerTypeZero {
			return nil
		}
		return p.NextDecoder(next)
	})
}
