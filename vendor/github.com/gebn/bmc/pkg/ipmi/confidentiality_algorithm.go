package ipmi

// ConfidentialityAlgorithm is the 6-bit identifier of an encryption algorithm
// used in the RMCP+ session establishment process. Packets with the encryption
// bit set in the session header are encrypted as per the specification for
// this algorithm.
type ConfidentialityAlgorithm uint8

const (
	ConfidentialityAlgorithmNone ConfidentialityAlgorithm = iota

	// ConfidentialityAlgorithmAESCBC128 specifies the use of AES-128-CBC (the
	// naming is to be consistent with the spec) for encrypted packets. The
	// confidentiality header in the IPMI payload is a 16-byte IV, randomly
	// generated for each message. The confidentiality trailer consists of a pad
	// of length between 0 and 15 to get the data to encrypt to be a multiple of
	// the algorithm block size (16), followed by the number of these bytes
	// added. The pad bytes start at 0x01, and increment each byte;
	// implementations must validate this. Support for this algorithm is
	// mandatory.
	ConfidentialityAlgorithmAESCBC128

	ConfidentialityAlgorithmXRC4128
	ConfidentialityAlgorithmXRC440
)

func (c ConfidentialityAlgorithm) String() string {
	switch c {
	case ConfidentialityAlgorithmNone:
		return "None"
	case ConfidentialityAlgorithmAESCBC128:
		return "AES-CBC-128"
	case ConfidentialityAlgorithmXRC4128:
		return "xRC4-128"
	case ConfidentialityAlgorithmXRC440:
		return "xRC4-40"
	}
	if c >= 0x30 && c <= 0x3f {
		return "OEM"
	}
	if c > 0x3f {
		// must fit into 6 bits, otherwise cannot be returned in Get Channel
		// Cipher Suites response
		return "Invalid"
	}
	return "Unknown"
}
