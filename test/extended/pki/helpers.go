package pki

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
)

// certInfo contains parsed certificate information
type certInfo struct {
	Algorithm string
	KeySize   int
	Curve     string
}

// pkiTestConfig defines a PKI test configuration
type pkiTestConfig struct {
	name            string
	algorithm       configv1alpha1.KeyAlgorithm
	rsaSize         int32
	ecdsaCurve      configv1alpha1.ECDSACurve
	signerOverride  *keyOverride
	servingOverride *keyOverride
}

// keyOverride specifies key configuration override for a certificate type
type keyOverride struct {
	algorithm  configv1alpha1.KeyAlgorithm
	rsaSize    int32
	ecdsaCurve configv1alpha1.ECDSACurve
}

// mixedPKITestConfig defines a mixed PKI test configuration with different settings per category
type mixedPKITestConfig struct {
	name              string
	signerAlgorithm   configv1alpha1.KeyAlgorithm
	signerRSASize     int32
	signerECDSACurve  configv1alpha1.ECDSACurve
	servingAlgorithm  configv1alpha1.KeyAlgorithm
	servingRSASize    int32
	servingECDSACurve configv1alpha1.ECDSACurve
	clientAlgorithm   configv1alpha1.KeyAlgorithm
	clientRSASize     int32
	clientECDSACurve  configv1alpha1.ECDSACurve
}

// operatorCertificate represents a certificate managed by an operator
type operatorCertificate struct {
	Namespace    string
	SecretName   string
	CertKey      string // Key in the secret containing the certificate (e.g., "tls.crt")
	Category     string // "signer", "serving", or "client"
	OperatorName string // For metrics verification
}

// waitForPKICRD waits for the PKI CRD to become available
func waitForPKICRD(ctx context.Context, configClient configclient.Interface, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := configClient.ConfigV1alpha1().PKIs().List(ctx, metav1.ListOptions{Limit: 1})
		if err != nil {
			// Only retry if the CRD is not yet registered (NotFound on the resource type)
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			// Return all other errors immediately (RBAC, transport, apiserver failures)
			return false, err
		}
		return true, nil
	})
}

// buildKeyConfig creates a KeyConfig from algorithm and key parameters
func buildKeyConfig(algorithm configv1alpha1.KeyAlgorithm, rsaSize int32, ecdsaCurve configv1alpha1.ECDSACurve) configv1alpha1.KeyConfig {
	keyConfig := configv1alpha1.KeyConfig{
		Algorithm: algorithm,
	}

	if algorithm == configv1alpha1.KeyAlgorithmRSA {
		keyConfig.RSA = configv1alpha1.RSAKeyConfig{
			KeySize: rsaSize,
		}
	} else if algorithm == configv1alpha1.KeyAlgorithmECDSA {
		keyConfig.ECDSA = configv1alpha1.ECDSAKeyConfig{
			Curve: ecdsaCurve,
		}
	}

	return keyConfig
}

// applyPKIConfig applies a PKI configuration based on the test config
func applyPKIConfig(ctx context.Context, configClient configclient.Interface, tc pkiTestConfig) error {
	// Build default key config (used for serving certs unless overridden)
	defaultKeyConfig := buildKeyConfig(tc.algorithm, tc.rsaSize, tc.ecdsaCurve)

	pkiProfile := configv1alpha1.PKIProfile{
		Defaults: configv1alpha1.DefaultCertificateConfig{
			Key: defaultKeyConfig,
		},
	}

	// Add signer certificate override if specified
	if tc.signerOverride != nil {
		signerKeyConfig := buildKeyConfig(tc.signerOverride.algorithm, tc.signerOverride.rsaSize, tc.signerOverride.ecdsaCurve)
		pkiProfile.SignerCertificates = configv1alpha1.CertificateConfig{
			Key: signerKeyConfig,
		}
	}

	// Add serving certificate override if specified
	if tc.servingOverride != nil {
		servingKeyConfig := buildKeyConfig(tc.servingOverride.algorithm, tc.servingOverride.rsaSize, tc.servingOverride.ecdsaCurve)
		pkiProfile.ServingCertificates = configv1alpha1.CertificateConfig{
			Key: servingKeyConfig,
		}
	}

	pki := &configv1alpha1.PKI{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1alpha1.PKISpec{
			CertificateManagement: configv1alpha1.PKICertificateManagement{
				Mode: configv1alpha1.PKICertificateManagementModeCustom,
				Custom: configv1alpha1.CustomPKIPolicy{
					PKIProfile: pkiProfile,
				},
			},
		},
	}

	// Try to create or update
	existing, err := configClient.ConfigV1alpha1().PKIs().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create new
			_, err = configClient.ConfigV1alpha1().PKIs().Create(ctx, pki, metav1.CreateOptions{})
			return err
		}
		// Return other errors (transient, permission, etc.)
		return err
	}

	// Update existing
	existing.Spec = pki.Spec
	_, err = configClient.ConfigV1alpha1().PKIs().Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

// applyMixedPKIConfig applies a mixed PKI configuration with different settings per category
func applyMixedPKIConfig(ctx context.Context, configClient configclient.Interface, tc mixedPKITestConfig) error {
	// Build default key config (we'll use signer as default)
	defaultKeyConfig := buildKeyConfig(tc.signerAlgorithm, tc.signerRSASize, tc.signerECDSACurve)

	// Build override configs for serving and client
	servingKeyConfig := buildKeyConfig(tc.servingAlgorithm, tc.servingRSASize, tc.servingECDSACurve)
	clientKeyConfig := buildKeyConfig(tc.clientAlgorithm, tc.clientRSASize, tc.clientECDSACurve)

	pki := &configv1alpha1.PKI{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1alpha1.PKISpec{
			CertificateManagement: configv1alpha1.PKICertificateManagement{
				Mode: configv1alpha1.PKICertificateManagementModeCustom,
				Custom: configv1alpha1.CustomPKIPolicy{
					PKIProfile: configv1alpha1.PKIProfile{
						Defaults: configv1alpha1.DefaultCertificateConfig{
							Key: defaultKeyConfig,
						},
						SignerCertificates: configv1alpha1.CertificateConfig{
							Key: defaultKeyConfig,
						},
						ServingCertificates: configv1alpha1.CertificateConfig{
							Key: servingKeyConfig,
						},
						ClientCertificates: configv1alpha1.CertificateConfig{
							Key: clientKeyConfig,
						},
					},
				},
			},
		},
	}

	// Try to create or update
	existing, err := configClient.ConfigV1alpha1().PKIs().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create new
			_, err = configClient.ConfigV1alpha1().PKIs().Create(ctx, pki, metav1.CreateOptions{})
			return err
		}
		// Return other errors (transient, permission, etc.)
		return err
	}

	// Update existing
	existing.Spec = pki.Spec
	_, err = configClient.ConfigV1alpha1().PKIs().Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

// getCertificateFromSecret retrieves and parses a certificate from a secret
func getCertificateFromSecret(ctx context.Context, kubeClient *kubernetes.Clientset, namespace, secretName, certKey string) (*certInfo, error) {
	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, secretName, err)
	}

	certData, ok := secret.Data[certKey]
	if !ok {
		return nil, fmt.Errorf("certificate key %q not found in secret %s/%s", certKey, namespace, secretName)
	}

	return parseCertificate(certData)
}

// parseCertificate parses PEM-encoded certificate data
func parseCertificate(certPEM []byte) (*certInfo, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	info := &certInfo{}

	switch pub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		info.Algorithm = "RSA"
		info.KeySize = pub.N.BitLen()
	case *ecdsa.PublicKey:
		info.Algorithm = "ECDSA"
		switch pub.Curve {
		case elliptic.P256():
			info.Curve = "P256"
		case elliptic.P384():
			info.Curve = "P384"
		case elliptic.P521():
			info.Curve = "P521"
		default:
			info.Curve = "Unknown"
		}
	default:
		return nil, fmt.Errorf("unsupported public key type: %T", pub)
	}

	return info, nil
}

// waitForSecretRegeneration waits for a secret to be recreated with new UID and populated cert data
// oldUID is the UID of the secret before deletion; certKey is the data key to verify (e.g., "tls.crt")
func waitForSecretRegeneration(ctx context.Context, kubeClient *kubernetes.Clientset, namespace, secretName, certKey string, oldUID string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		s, err := kubeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			// Only retry if the secret doesn't exist yet (expected during regeneration)
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			// Return all other errors immediately (RBAC, transport, apiserver failures)
			return false, err
		}
		if string(s.UID) == oldUID {
			return false, nil // Still the old secret (delete not propagated yet)
		}
		// Verify the cert data is populated
		_, ok := s.Data[certKey]
		return ok, nil // Return true only if new secret with populated cert data
	})
}

// deleteCertificateSecret deletes a certificate secret to trigger rotation/regeneration
func deleteCertificateSecret(ctx context.Context, kubeClient *kubernetes.Clientset, namespace, secretName string) error {
	return kubeClient.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
}

// cleanupPKIConfiguration resets the PKI configuration to default (Unmanaged)
// NOTE: Does NOT disable the feature gate - feature gate lifecycle is managed by CI job config
func cleanupPKIConfiguration(ctx context.Context, configClient configclient.Interface) {
	e2e.Logf("Starting PKI cleanup...")

	// Reset PKI cluster resource to default (unmanaged) configuration
	e2e.Logf("Resetting PKI cluster resource to default configuration...")
	pki, err := configClient.ConfigV1alpha1().PKIs().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			e2e.Logf("Warning: error getting PKI resource: %v", err)
		} else {
			e2e.Logf("PKI resource not found, skipping reset")
		}
	} else {
		// Reset to default/unmanaged mode
		// Note: custom field must be empty when mode is Unmanaged
		pki.Spec.CertificateManagement.Mode = configv1alpha1.PKICertificateManagementModeUnmanaged
		pki.Spec.CertificateManagement.Custom = configv1alpha1.CustomPKIPolicy{}

		_, err = configClient.ConfigV1alpha1().PKIs().Update(ctx, pki, metav1.UpdateOptions{})
		if err != nil {
			e2e.Logf("Warning: error resetting PKI resource: %v", err)
		} else {
			e2e.Logf("✓ PKI cluster resource reset to Unmanaged mode successfully")
		}
	}

	e2e.Logf("PKI cleanup completed")
}
