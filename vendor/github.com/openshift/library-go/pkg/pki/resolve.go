package pki

import (
	"fmt"

	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	"github.com/openshift/library-go/pkg/crypto"
)

// CertificateConfig holds the resolved configuration for a specific certificate.
// Currently contains key configuration; will grow as the PKI API expands to
// include additional certificate properties.
type CertificateConfig struct {
	// Key is the resolved key pair generator.
	Key crypto.KeyPairGenerator
}

// ResolveCertificateConfig resolves the effective certificate configuration
// for a given certificate type and name from the PKI profile.
//
// Returns nil if the provider returns a nil profile (Unmanaged mode),
// indicating that the caller should use its own default behavior.
//
// The name parameter is reserved for future per-certificate overrides and
// can be used for metrics and logging.
func ResolveCertificateConfig(provider PKIProfileProvider, certType CertificateType, name string) (*CertificateConfig, error) {
	profile, err := provider.PKIProfile()
	if err != nil {
		return nil, fmt.Errorf("resolving PKI profile for %s certificate %q: %w", certType, name, err)
	}
	if profile == nil {
		return nil, nil
	}

	switch certType {
	case CertificateTypeSigner:
		return resolveKeyConfig(profile.Defaults, profile.SignerCertificates)
	case CertificateTypeServing:
		return resolveKeyConfig(profile.Defaults, profile.ServingCertificates)
	case CertificateTypeClient:
		return resolveKeyConfig(profile.Defaults, profile.ClientCertificates)
	case CertificateTypePeer:
		return resolvePeerKeyConfig(profile)
	default:
		return nil, fmt.Errorf("unknown certificate type: %q", certType)
	}
}

// resolveKeyConfig returns the override KeyConfig if its Algorithm is set,
// otherwise falls back to the default.
func resolveKeyConfig(defaults configv1alpha1.DefaultCertificateConfig, override configv1alpha1.CertificateConfig) (*CertificateConfig, error) {
	apiKey := defaults.Key
	if override.Key.Algorithm != "" {
		apiKey = override.Key
	}
	g, err := KeyPairGeneratorFromAPI(apiKey)
	if err != nil {
		return nil, err
	}
	return &CertificateConfig{Key: g}, nil
}

// resolvePeerKeyConfig resolves both the serving and client configs and
// returns whichever has higher NIST security strength.
func resolvePeerKeyConfig(profile *configv1alpha1.PKIProfile) (*CertificateConfig, error) {
	servingCfg, err := resolveKeyConfig(profile.Defaults, profile.ServingCertificates)
	if err != nil {
		return nil, fmt.Errorf("resolving serving config for peer: %w", err)
	}
	clientCfg, err := resolveKeyConfig(profile.Defaults, profile.ClientCertificates)
	if err != nil {
		return nil, fmt.Errorf("resolving client config for peer: %w", err)
	}
	return &CertificateConfig{
		Key: strongerKeyPairGenerator(servingCfg.Key, clientCfg.Key),
	}, nil
}
