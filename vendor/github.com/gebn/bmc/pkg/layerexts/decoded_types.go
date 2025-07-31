package layerexts

import (
	"fmt"

	"github.com/google/gopacket"
)

// DecodedTypes represents a sequence of layers in the order they were decoded.
//
// Originally, this was a struct containing a slice of layers in the order they
// were decoded, along with a map[gopacket.LayerType]gopacket.Layer. The idea
// was the slice provided layer order (answering questions like "What is the
// inner-most layer?") and the map provides fast lookup ("Is this layer present
// in the response?"). There were several issues however. The map required a
// fresh allocation per packet decoded (morphing the previous map into the new
// one would be too inefficient), the layer returned by the map lookup was of
// the wrong type, and users of the map had access to a copy in the right type
// anyway, so this was not useful. It also turns out searching through a slice
// is quicker than a map lookup for sizes <5 or so, and in practice we never
// have more than 5 layers in a packet. Hence, the map was discarded.
type DecodedTypes []gopacket.LayerType

// Contains ensures the provided layer is present. If not, it returns an
// appropriate error.
func (ts DecodedTypes) Contains(needle gopacket.LayerType) error {
	// search backwards, as the most common use case is looking for inner layers
	for i := len(ts) - 1; i >= 0; i-- {
		if ts[i] == needle {
			return nil
		}
	}
	return fmt.Errorf("%v layer not received", needle)
}

// InnermostEquals ensures the inner-most layer, i.e. the last one decoded, is
// of a particular type. If it is not, an error is returned.
func (ts DecodedTypes) InnermostEquals(want gopacket.LayerType) error {
	if len(ts) == 0 {
		return fmt.Errorf("no layers received")
	}
	if got := ts[len(ts)-1]; got != want {
		return fmt.Errorf("inner-most layer is %v, wanted %v", got, want)
	}
	return nil
}
