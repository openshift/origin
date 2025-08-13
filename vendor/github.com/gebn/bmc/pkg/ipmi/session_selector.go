package ipmi

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func init() {
	layers.RegisterRMCPLayerType(layers.RMCPClassIPMI, LayerTypeSessionSelector)
}

// SessionSelector is a dummy layer that lives between the RMCP layer and the
// IPMI session wrapper. It allows us to choose the correct session wrapper
// layer based on the AuthType field, which indicates whether the packet is
// using v1.5 or v2.0 sessions. Gopacket's RMCP layer allows registering a
// single layer for the IPMI message class, which isn't flexible enough for what
// we really need. Hence the existence of this layer, which is 0-length. Ideally
// this logic would go in gopacket, however we have not contributed those types
// back to the library as they are a little too specific.
type SessionSelector struct {
	layers.BaseLayer

	// IsRMCPPlus indicates whether the payload of this layer is an IPMI v2.0
	// session wrapper (true), or a v1.5 session wrapper (false).
	IsRMCPPlus bool
}

func (*SessionSelector) LayerType() gopacket.LayerType {
	return LayerTypeSessionSelector
}

func (s *SessionSelector) CanDecode() gopacket.LayerClass {
	return s.LayerType()
}

func (s *SessionSelector) NextLayerType() gopacket.LayerType {
	if s.IsRMCPPlus {
		return LayerTypeV2Session
	}
	return LayerTypeV1Session
}

func (s *SessionSelector) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	if len(data) < 1 {
		df.SetTruncated()
		return fmt.Errorf("session wrapper cannot be empty")
	}

	s.BaseLayer.Payload = data // the session selector layer has length 0
	s.IsRMCPPlus = AuthenticationType(data[0]) == AuthenticationTypeRMCPPlus
	return nil
}
