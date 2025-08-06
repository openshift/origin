package ipmi

// IntegrityAlgorithm is the 6-bit identifier of an integrity algorithm
// negotiated during the RMCP+ session establishment process. The numbers are
// defined in 13.28.4 of the spec. The integrity algorithm is used to calculate
// the signature for authenticated RMCP+ messages.
type IntegrityAlgorithm uint8

const (
	IntegrityAlgorithmNone          IntegrityAlgorithm = iota
	IntegrityAlgorithmHMACSHA196                       // 12 byte authcode
	IntegrityAlgorithmHMACMD5128                       // 16 bytes ''
	IntegrityAlgorithmMD5128                           // 16 bytes ''
	IntegrityAlgorithmHMACSHA256128                    // 16 bytes ''
)

func (i IntegrityAlgorithm) String() string {
	switch i {
	case IntegrityAlgorithmNone:
		return "None"
	case IntegrityAlgorithmHMACSHA196:
		return "HMAC-SHA1-96"
	case IntegrityAlgorithmHMACMD5128:
		return "HMAC-MD5-128"
	case IntegrityAlgorithmMD5128:
		return "MD5-128"
	case IntegrityAlgorithmHMACSHA256128:
		return "HMAC-SHA256-128"
	}
	if i >= 0xc0 && i <= 0x3f {
		return "OEM"
	}
	if i > 0x3f {
		// must fit into 6 bits, otherwise cannot be returned in Get Channel
		// Cipher Suites response
		return "Invalid"
	}
	return "Unknown"
}
