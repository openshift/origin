package ipmi

import (
	"fmt"

	"github.com/gebn/bmc/pkg/iana"

	"github.com/google/gopacket"
)

// PayloadDescriptor contains RMCP+ session fields which, taken together, describe the
// format of a IPMI v2.0 session payload. V2Session embeds this type. This type
// does not appear in the specification.
type PayloadDescriptor struct {

	// PayloadType identifies the payload, e.g. an IPMI or RAKP message. When
	// this has a value of OEM (0x2), it must be used together with the
	// Enterprise and PayloadID fields to identify the format.
	PayloadType PayloadType

	// Enterprise is the IANA Enterprise Number of the OEM who describes the
	// payload. This field only exists on the wire if the payload type is OEM
	// explicit.
	Enterprise iana.Enterprise

	// PayloadID identifies the payload within the Enterprise when the payload
	// is OEM-defined. This field only exists on the wire if the payload type is
	// OEM explicit.
	PayloadID uint16
}

var (
	PayloadDescriptorIPMI = PayloadDescriptor{
		PayloadType: PayloadTypeIPMI,
	}
	PayloadDescriptorOpenSessionReq = PayloadDescriptor{
		PayloadType: PayloadTypeOpenSessionReq,
	}
	PayloadDescriptorOpenSessionRsp = PayloadDescriptor{
		PayloadType: PayloadTypeOpenSessionRsp,
	}
	PayloadDescriptorRAKPMessage1 = PayloadDescriptor{
		PayloadType: PayloadTypeRAKPMessage1,
	}
	PayloadDescriptorRAKPMessage2 = PayloadDescriptor{
		PayloadType: PayloadTypeRAKPMessage2,
	}
	PayloadDescriptorRAKPMessage3 = PayloadDescriptor{
		PayloadType: PayloadTypeRAKPMessage3,
	}
	PayloadDescriptorRAKPMessage4 = PayloadDescriptor{
		PayloadType: PayloadTypeRAKPMessage4,
	}

	payloadLayerTypes = map[PayloadDescriptor]gopacket.LayerType{
		PayloadDescriptorIPMI:           LayerTypeMessage,
		PayloadDescriptorOpenSessionReq: LayerTypeOpenSessionReq,
		PayloadDescriptorOpenSessionRsp: LayerTypeOpenSessionRsp,
		PayloadDescriptorRAKPMessage1:   LayerTypeRAKPMessage1,
		PayloadDescriptorRAKPMessage2:   LayerTypeRAKPMessage2,
		PayloadDescriptorRAKPMessage3:   LayerTypeRAKPMessage3,
		PayloadDescriptorRAKPMessage4:   LayerTypeRAKPMessage4,
	}
)

func (p PayloadDescriptor) NextLayerType() gopacket.LayerType {
	if layer, ok := payloadLayerTypes[p]; ok {
		return layer
	}
	return gopacket.LayerTypePayload
}

func (p PayloadDescriptor) String() string {
	switch p.PayloadType {
	case PayloadTypeOEM:
		return fmt.Sprintf("PayloadDescriptor(OEM, %v, %#x", p.Enterprise, p.PayloadID)
	default:
		return fmt.Sprintf("PayloadDescriptor(%v)", p.PayloadType)
	}
}

// RegisterOEMPayloadDescriptor adds or overrides how an IPMI v2.0 OEM payload
// is handled within a session. This is implemented via a map, so care must be
// taken to not call this function in parallel.
func RegisterOEMPayloadDescriptor(enterprise iana.Enterprise, payloadID uint16, LayerType gopacket.LayerType) {
	payload := PayloadDescriptor{
		PayloadType: PayloadTypeOEM,
		Enterprise:  enterprise,
		PayloadID:   payloadID,
	}
	payloadLayerTypes[payload] = LayerType
}
