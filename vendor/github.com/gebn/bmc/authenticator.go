package bmc

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"

	"github.com/gebn/bmc/pkg/ipmi"
)

// truncatedHash truncates a hash.Hash's output. This is used to implement
// algorithms like HMAC-SHA1-96 (the first 12 bytes of HMAC-SHA1), and
// HMAC-SHA256-128 (the first 16 bytes of HMAC-SHA256).
type truncatedHash struct {
	hash.Hash
	length int
}

func (t truncatedHash) Sum(b []byte) []byte {
	sum := t.Hash.Sum(b)
	return sum[:len(b)+t.length]
}

func (t truncatedHash) Size() int {
	return t.length
}

// authenticationAlgorithmParams produces Hash implementations to generate
// various values. It contains the configurable parameters of the session
// establishment authentication algorithm. An instance of this struct can be
// created from an authentication algorithm alone, and means we can treat all
// algorithms identically from the point it is obtained.
type authenticationAlgorithmParams struct {

	// hashGen is a function returning the underlying hash algorithm of the
	// HMAC.
	hashGen func() hash.Hash

	// icvLength is the length to truncate integrity check values to. This is
	// only used to generate the HMAC for RAKP Message 4. A value of 0 means no
	// truncation.
	icvLength int
}

// AuthCode returns a hash.Hash implementation for producing and verifying the
// AuthCodes in RAKP messages 2 and 3. All material required to generate these
// values is available in RAKP messages 1 and 2.
func (g *authenticationAlgorithmParams) AuthCode(kuid []byte) hash.Hash {
	return hmac.New(g.hashGen, kuid)
}

// SIK returns a hash.Hash implementation for producing the SIK from RAKP
// messages 1 and 2.
func (g *authenticationAlgorithmParams) SIK(kg []byte) hash.Hash {
	return hmac.New(g.hashGen, kg)
}

// K returns a hash.Hash implementation for creating additional key material,
// also referred to as K_N.
func (g *authenticationAlgorithmParams) K(sik []byte) hash.Hash {
	return hmac.New(g.hashGen, sik)
}

// ICV returns a hash.Hash implementation for validating the ICV field in RAKP
// Message 4. The inputs to this hash are contained in RAKP messages 1 and 2.
func (g *authenticationAlgorithmParams) ICV(sik []byte) hash.Hash {
	if g.icvLength == 0 {
		return g.K(sik)
	}
	return truncatedHash{
		Hash:   g.K(sik),
		length: g.icvLength,
	}
}

// algorithmAuthenticationHashGenerator builds an authenticator for the
// specified algorithm, using the provided key. This authenticator can then be
// used in the RAKP session establishment process.
func algorithmAuthenticationHashGenerator(a ipmi.AuthenticationAlgorithm) (*authenticationAlgorithmParams, error) {
	switch a {
	// TODO support ipmi.AuthenticationAlgorithmNone - this is difficult as we
	// need to create a bunch of valid but completely useless structs...
	case ipmi.AuthenticationAlgorithmHMACSHA1:
		return &authenticationAlgorithmParams{
			hashGen:   sha1.New,
			icvLength: 12,
		}, nil
	case ipmi.AuthenticationAlgorithmHMACSHA256:
		return &authenticationAlgorithmParams{
			hashGen:   sha256.New,
			icvLength: 16,
		}, nil
	case ipmi.AuthenticationAlgorithmHMACMD5:
		return &authenticationAlgorithmParams{
			hashGen: md5.New, // ICV not truncated
		}, nil
	default:
		return nil, fmt.Errorf("unknown authentication algorithm: %v", a)
	}
}

// authenticator is not as simple as hash([]byte) []byte - the input array must
// be constructed manually by serialising fields from various packets

// executeHash is a convenience function to calculate the hash of a slice of
// bytes. It leaves the hash in a reset state. Note that this function cannot be
// called concurrently on a single underlying hash.Hash.
func executeHash(h hash.Hash, b []byte) []byte {
	if h == nil {
		return nil
	}
	h.Write(b)
	sum := h.Sum(nil)
	h.Reset()
	return sum
}

// AdditionalKeyMaterialGenerator is satisfied by types that can produce key
// material derived from the Session Integrity Key, as defined in section 13.32
// of IPMI v2.0. This additional key material is referred to as K_N. In
// practice, only K_1 and K_2 are used, for packet authentication and
// confidentiality respectively
type AdditionalKeyMaterialGenerator interface {

	// K computes the value of K_N for a given value of N, using the negotiated
	// authentication algorithm (used during session establishment) loaded with
	// the SIK. N is only defined for values 1 through 255. This method is not
	// used by the library itself, and is assumed to be only for
	// informational/debugging purposes, so we make no attempt to memoise
	// results. This function is not safe for concurrent use by multiple
	// goroutines.
	K(n int) []byte
}

type additionalKeyMaterialGenerator struct {
	hash hash.Hash
}

func (g additionalKeyMaterialGenerator) K(n int) []byte {
	// kConstantLength is the length of constant to compute the MAC of using
	// the SIK. "These constants are constructed using a hexadecimal octet
	// value repeated up to the HMAC block size in length" in 13.32 of the spec
	// is misleading: the length is always 20, regardless of the underlying
	// algorithm's block size. See #49 for a bug this caused.
	const kConstantLength = 20

	constant := make([]byte, kConstantLength)
	for i := 0; i < kConstantLength; i++ {
		constant[i] = uint8(n)
	}
	return executeHash(g.hash, constant)
}

func calculateSIK(h hash.Hash, rakpMessage1 *ipmi.RAKPMessage1, rakpMessage2 *ipmi.RAKPMessage2) []byte {
	h.Write(rakpMessage1.RemoteConsoleRandom[:]) // R_M
	h.Write(rakpMessage2.ManagedSystemRandom[:]) // R_C
	role := uint8(rakpMessage1.MaxPrivilegeLevel)
	if !rakpMessage1.PrivilegeLevelLookup {
		role |= 1 << 4
	}
	h.Write([]byte{role})                              // Role_M (entire byte from original wire format)
	h.Write([]byte{uint8(len(rakpMessage1.Username))}) // ULength_M
	h.Write([]byte(rakpMessage1.Username))             // UName_M
	sum := h.Sum(nil)
	h.Reset()
	return sum
}

// calculateRAKPMessage2AuthCode computes the ICV that should be sent by the BMC
// in RAKP Message 2 based on the RAKP Message 1 sent by the remote console and
// the RAKP Message 2 sent by the BMC.
func calculateRAKPMessage2AuthCode(h hash.Hash, rakpMessage1 *ipmi.RAKPMessage1, rakpMessage2 *ipmi.RAKPMessage2) []byte {
	buf := [4]byte{}

	// session IDs are in wire byte order, presumably for efficiency, but we'd
	// rather decode and re-encode for the sake of code organisation
	binary.LittleEndian.PutUint32(buf[:], rakpMessage2.RemoteConsoleSessionID)
	h.Write(buf[:]) // SID_M
	binary.LittleEndian.PutUint32(buf[:], rakpMessage1.ManagedSystemSessionID)
	h.Write(buf[:]) // SID_C

	h.Write(rakpMessage1.RemoteConsoleRandom[:]) // R_M
	h.Write(rakpMessage2.ManagedSystemRandom[:]) // R_C
	h.Write(rakpMessage2.ManagedSystemGUID[:])   // GUID_C
	role := uint8(rakpMessage1.MaxPrivilegeLevel)
	if !rakpMessage1.PrivilegeLevelLookup {
		role |= 1 << 4
	}
	h.Write([]byte{role})                              // Role_M (entire byte from original wire format)
	h.Write([]byte{uint8(len(rakpMessage1.Username))}) // ULength_M
	h.Write([]byte(rakpMessage1.Username))             // UName_M
	sum := h.Sum(nil)
	h.Reset()
	return sum
}

func calculateRAKPMessage3AuthCode(h hash.Hash, rakpMessage1 *ipmi.RAKPMessage1, rakpMessage2 *ipmi.RAKPMessage2) []byte {
	h.Write(rakpMessage2.ManagedSystemRandom[:]) // R_C
	buf := [4]byte{}
	binary.LittleEndian.PutUint32(buf[:], rakpMessage2.RemoteConsoleSessionID)
	h.Write(buf[:]) // SID_M
	role := uint8(rakpMessage1.MaxPrivilegeLevel)
	if !rakpMessage1.PrivilegeLevelLookup {
		role |= 1 << 4
	}
	h.Write([]byte{role})                              // Role_M (entire byte from original wire format)
	h.Write([]byte{uint8(len(rakpMessage1.Username))}) // ULength_M
	h.Write([]byte(rakpMessage1.Username))             // UName_M
	sum := h.Sum(nil)
	h.Reset()
	return sum
}

func calculateRAKPMessage4ICV(h hash.Hash, rakpMessage1 *ipmi.RAKPMessage1, rakpMessage2 *ipmi.RAKPMessage2) []byte {
	h.Write(rakpMessage1.RemoteConsoleRandom[:]) // R_M
	buf := [4]byte{}
	binary.LittleEndian.PutUint32(buf[:], rakpMessage1.ManagedSystemSessionID)
	h.Write(buf[:])                            // SID_C
	h.Write(rakpMessage2.ManagedSystemGUID[:]) // GUID_C
	sum := h.Sum(nil)
	h.Reset()
	return sum
}
