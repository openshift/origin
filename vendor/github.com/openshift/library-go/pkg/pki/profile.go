package pki

import (
	"fmt"

	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	"github.com/openshift/library-go/pkg/crypto"
)

// DefaultPKIProfile returns the default PKIProfile for OpenShift.
func DefaultPKIProfile() configv1alpha1.PKIProfile {
	return configv1alpha1.PKIProfile{
		Defaults: configv1alpha1.DefaultCertificateConfig{
			Key: configv1alpha1.KeyConfig{
				Algorithm: configv1alpha1.KeyAlgorithmECDSA,
				ECDSA:     configv1alpha1.ECDSAKeyConfig{Curve: configv1alpha1.ECDSACurveP256},
			},
		},
		SignerCertificates: configv1alpha1.CertificateConfig{
			Key: configv1alpha1.KeyConfig{
				Algorithm: configv1alpha1.KeyAlgorithmECDSA,
				ECDSA:     configv1alpha1.ECDSAKeyConfig{Curve: configv1alpha1.ECDSACurveP384},
			},
		},
	}
}

// KeyPairGeneratorFromAPI converts a configv1alpha1.KeyConfig to a
// crypto.KeyPairGenerator.
func KeyPairGeneratorFromAPI(apiKey configv1alpha1.KeyConfig) (crypto.KeyPairGenerator, error) {
	switch apiKey.Algorithm {
	case configv1alpha1.KeyAlgorithmRSA:
		return crypto.RSAKeyPairGenerator{
			Bits: int(apiKey.RSA.KeySize),
		}, nil
	case configv1alpha1.KeyAlgorithmECDSA:
		curve, err := ecdsaCurveFromAPI(apiKey.ECDSA.Curve)
		if err != nil {
			return nil, err
		}
		return crypto.ECDSAKeyPairGenerator{
			Curve: curve,
		}, nil
	default:
		return nil, fmt.Errorf("unknown key algorithm: %q", apiKey.Algorithm)
	}
}

// ecdsaCurveFromAPI converts an API ECDSA curve name to the crypto package's ECDSACurve.
func ecdsaCurveFromAPI(c configv1alpha1.ECDSACurve) (crypto.ECDSACurve, error) {
	switch c {
	case configv1alpha1.ECDSACurveP256:
		return crypto.P256, nil
	case configv1alpha1.ECDSACurveP384:
		return crypto.P384, nil
	case configv1alpha1.ECDSACurveP521:
		return crypto.P521, nil
	default:
		return "", fmt.Errorf("unknown ECDSA curve: %q", c)
	}
}

// securityBits returns the NIST security strength in bits for a given
// KeyPairGenerator. For RSA, values come from the rsaSecurityStrength table.
// For ECDSA, security strength is half the key size (fixed per curve).
func securityBits(g crypto.KeyPairGenerator) int {
	switch g := g.(type) {
	case crypto.RSAKeyPairGenerator:
		return rsaSecurityStrength[g.Bits]
	case crypto.ECDSAKeyPairGenerator:
		switch g.Curve {
		case crypto.P256:
			return 128
		case crypto.P384:
			return 192
		case crypto.P521:
			return 256
		}
	}
	return 0
}

// rsaSecurityStrength maps RSA key sizes (2048-8192 in 1024-bit increments)
// to their security strengths from NIST SP 800-56B Rev 2 Table 2 or
// pre-calculated from the GNFS complexity estimate.
var rsaSecurityStrength = map[int]int{
	2048: 112,
	3072: 128,
	4096: 152,
	5120: 168,
	6144: 176,
	7168: 192,
	8192: 200,
}

// strongerKeyPairGenerator returns whichever of a or b provides higher NIST
// security strength. In case of a tie, ECDSA is preferred over RSA.
func strongerKeyPairGenerator(a, b crypto.KeyPairGenerator) crypto.KeyPairGenerator {
	sa, sb := securityBits(a), securityBits(b)
	if sb > sa {
		return b
	}
	if sa > sb {
		return a
	}
	// Equal strength: prefer ECDSA over RSA.
	if _, ok := a.(crypto.ECDSAKeyPairGenerator); ok {
		return a
	}
	if _, ok := b.(crypto.ECDSAKeyPairGenerator); ok {
		return b
	}
	return a
}
