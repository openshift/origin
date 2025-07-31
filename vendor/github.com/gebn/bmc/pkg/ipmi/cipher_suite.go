package ipmi

import (
	"fmt"

	"github.com/gebn/bmc/pkg/iana"
)

// CipherSuite represents the authentication, integrity and confidentiality
// triple of algorithms used to establish a session. Each BMC supports one or
// more sets of these, the most common being 3 and 17. The default value is
// equivalent to Cipher Suite 0, which should be disabled on all BMCs. This
// struct must be comparable.
type CipherSuite struct {
	AuthenticationAlgorithm
	IntegrityAlgorithm
	ConfidentialityAlgorithm
}

var (
	// CipherSuite3 represents Cipher Suite 3 (RAKP-HMAC-SHA1/HMAC-SHA1-96/AES-CBC-128),
	// which must be supported by all IPMI v2.0 BMCs as the underlying algorithms are
	// each marked as mandatory in the spec.
	CipherSuite3 = CipherSuite{
		AuthenticationAlgorithmHMACSHA1,
		IntegrityAlgorithmHMACSHA196,
		ConfidentialityAlgorithmAESCBC128,
	}

	// CipherSuite17 represents Cipher Suite 17 (RAKP-HMAC-SHA256/HMAC-SHA256-128/AES-CBC-128),
	// which is supported by newer BMCs.
	CipherSuite17 = CipherSuite{
		AuthenticationAlgorithmHMACSHA256,
		IntegrityAlgorithmHMACSHA256128,
		ConfidentialityAlgorithmAESCBC128,
	}
)

func (c CipherSuite) String() string {
	return fmt.Sprintf("%v/%v/%v",
		c.AuthenticationAlgorithm,
		c.IntegrityAlgorithm,
		c.ConfidentialityAlgorithm)
}

// CipherSuiteID represents the 8-bit numeric identity of a cipher suite. There
// are currently 20 standard cipher suites (0-19), with the most common being
// 3, for which support is mandatory, and 17. Higher identities are not
// necessarily more secure.
type CipherSuiteID uint8

func (c CipherSuiteID) Description() string {
	// 8 bits on the wire, so no values are inherently invalid, just currently
	// undefined
	switch {
	case c < 20:
		return "Standard"
	case c >= 0x80 && c <= 0xbf:
		return "OEM"
	default:
		return "Unknown"
	}
}

func (c CipherSuiteID) String() string {
	return fmt.Sprintf("%v(%v)", uint8(c), c.Description())
}

// CipherSuiteRecord represents an identified trio of algorithms, plus IANA
// enterprise number if OEM-specific. While an OEM can implement a single suite
// ID supporting multiple integrity and confidentiality algorithms, this has
// not been observed, and can be represented by multiple instances of this
// struct.
type CipherSuiteRecord struct {
	CipherSuiteID
	CipherSuite

	// Enterprise is 0 if the cipher suite is standard.
	iana.Enterprise
}
