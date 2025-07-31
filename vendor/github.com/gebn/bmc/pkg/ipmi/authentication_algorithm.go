package ipmi

// AuthenticationAlgorithm is the 6-bit identifier of an authentication
// algorithm used in the RMCP+ session establishment process. It has no use
// once the session is active. The numbers are defined in 13.28 of the spec.
// IPMI v1.5's equivalent is authentication type.
type AuthenticationAlgorithm uint8

const (
	// AuthenticationAlgorithmNone is equivalent to
	// AuthenticationAlgorithmHMACSHA1, however the key exchange authentication
	// code fields in RAKP2 and 3, and the ICV field in RAKP4 are absent.
	// Password auth and packet level integrity checking are unavailable. The
	// privilege level is established using only the username/role (the former
	// of which may be null, with a null password, allowing anonymous access).
	// Support for this algorithm is mandatory.
	AuthenticationAlgorithmNone AuthenticationAlgorithm = iota

	// AuthenticationAlgorithmHMACSHA1 specifies that HMAC-SHA1 (RFC2104) is
	// used to create 20-byte key exchange authentication code fields in RAKP2
	// and RAKP3. HMAC-SHA1-96 (RFC2404) is used for generating a 12-byte ICV in
	// RAKP4. Support for this algorithm is mandatory.
	AuthenticationAlgorithmHMACSHA1

	// AuthenticationAlgorithmHMACMD5 specifies that HMAC-MD5 (RFC2104) is used
	// to create 16-byte key exchange authentication codes in RAKP2 and RAKP3,
	// and ICV in RAKP4.
	AuthenticationAlgorithmHMACMD5

	// AuthenticationAlgorithmHMACSHA256 specifies that HMAC-SHA256 (FIPS 180-2,
	// RFC4634) is used to create 32-byte key exchange authentication code
	// fields in RAKP2 and RAKP3. HMAC-SHA256-128 (RFC4868) is used for
	// generating a 12-byte ICV in RAKP4.
	AuthenticationAlgorithmHMACSHA256
)

func (a AuthenticationAlgorithm) String() string {
	switch a {
	case AuthenticationAlgorithmNone:
		return "None"
	case AuthenticationAlgorithmHMACSHA1:
		return "RAKP-HMAC-SHA1"
	case AuthenticationAlgorithmHMACMD5:
		return "RAKP-HMAC-MD5"
	case AuthenticationAlgorithmHMACSHA256:
		return "RAKP-HMAC-SHA256"
	}
	if a >= 0xc0 && a <= 0x3f {
		return "OEM"
	}
	if a > 0x3f {
		// must fit into 6 bits, otherwise cannot be returned in Get Channel
		// Cipher Suites response
		return "Invalid"
	}
	return "Unknown"
}
