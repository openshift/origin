package ipmi

// BodyCode is the type of defining body codes, used to indicate the structure
// of request and response data when the network function is Group Extension
// (0x2c,0x2d). See section 5.1 in the v1.5 and v2.0 specifications. This value
// becomes the first byte of IPMI message data, however in this implementation
// it is part of the message to ease selection of the data layer type.
type BodyCode uint8

const (
	BodyCodePICMG BodyCode = 0x00
	BodyCodeDMTF  BodyCode = 0x01
	BodyCodeSSI   BodyCode = 0x02
	BodyCodeVSO   BodyCode = 0x03
	BodyCodeDCMI  BodyCode = 0xdc
)

func (b BodyCode) String() string {
	switch b {
	case BodyCodePICMG:
		return "PCI Industrial Computer Manufacturer's Group"
	case BodyCodeDMTF:
		return "DMTF Pre-OS Working Group ASF Specification"
	case BodyCodeSSI:
		return "Server System Infrastructure (SSI) Forum"
	case BodyCodeVSO:
		return "VITA Standards Organization (VSO)"
	case BodyCodeDCMI:
		return "DCMI"
	default:
		return "Unknown" // reserved, or invalid
	}
}
