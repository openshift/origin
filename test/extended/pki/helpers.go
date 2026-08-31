package pki

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
)

// certInfo contains parsed certificate information
type certInfo struct {
	Algorithm string
	KeySize   int
	Curve     string
	IsCA      bool
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
	Namespace  string
	SecretName string
	CertKey    string // Key in the secret containing the certificate (e.g., "tls.crt")
	Category   string // "signer", "serving", or "client"
}

// verifyCertIsCA asserts the CA flag matches the certificate category: signer
// certificates must be CAs, serving/client certificates must not be.
func verifyCertIsCA(cert *certInfo, category, namespace, secretName string) {
	if category == "signer" {
		o.Expect(cert.IsCA).To(o.BeTrue(), "signer certificate %s/%s should be a CA", namespace, secretName)
		return
	}
	o.Expect(cert.IsCA).To(o.BeFalse(), "%s certificate %s/%s should not be a CA (leaf-first PEM ordering may have changed)", category, namespace, secretName)
}

// verifyCertKeyConfig asserts a regenerated certificate matches the expected key
// algorithm and parameters. Unknown/empty algorithms fail the test so a new
// algorithm (e.g. Ed25519) cannot pass unverified.
func verifyCertKeyConfig(cert *certInfo, algorithm configv1alpha1.KeyAlgorithm, rsaSize int32, ecdsaCurve configv1alpha1.ECDSACurve, category, namespace, secretName string) {
	switch algorithm {
	case configv1alpha1.KeyAlgorithmRSA:
		o.Expect(cert.Algorithm).To(o.Equal("RSA"), "expected RSA algorithm for %s certificate %s/%s", category, namespace, secretName)
		o.Expect(int32(cert.KeySize)).To(o.Equal(rsaSize), "expected RSA key size %d for %s certificate %s/%s", rsaSize, category, namespace, secretName)
		e2e.Logf("    %s certificate verified: RSA-%d", category, cert.KeySize)
	case configv1alpha1.KeyAlgorithmECDSA:
		o.Expect(cert.Algorithm).To(o.Equal("ECDSA"), "expected ECDSA algorithm for %s certificate %s/%s", category, namespace, secretName)
		expectedCurve := string(ecdsaCurve)
		o.Expect(cert.Curve).To(o.Equal(expectedCurve), "expected ECDSA curve %s for %s certificate %s/%s", expectedCurve, category, namespace, secretName)
		e2e.Logf("    %s certificate verified: ECDSA-%s", category, cert.Curve)
	default:
		g.Fail(fmt.Sprintf("unexpected key algorithm %q for %s certificate %s/%s", algorithm, category, namespace, secretName))
	}
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

	pkiProfile := configv1alpha1.PKIProfile{
		Defaults: configv1alpha1.DefaultCertificateConfig{
			Key: defaultKeyConfig,
		},
		SignerCertificates: configv1alpha1.CertificateConfig{
			Key: defaultKeyConfig,
		},
	}

	// Only configure serving/client profiles when requested; leaving an unset
	// category out keeps the applied spec in sync with what the test validates.
	if tc.servingAlgorithm != "" {
		pkiProfile.ServingCertificates = configv1alpha1.CertificateConfig{
			Key: buildKeyConfig(tc.servingAlgorithm, tc.servingRSASize, tc.servingECDSACurve),
		}
	}
	if tc.clientAlgorithm != "" {
		pkiProfile.ClientCertificates = configv1alpha1.CertificateConfig{
			Key: buildKeyConfig(tc.clientAlgorithm, tc.clientRSASize, tc.clientECDSACurve),
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

// getCertificateFromSecret retrieves and parses a certificate from a secret
func getCertificateFromSecret(ctx context.Context, kubeClient kubernetes.Interface, namespace, secretName, certKey string) (*certInfo, error) {
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

// parseCertificate parses PEM-encoded certificate data.
// Only the first PEM block is decoded: for serving/client certs this relies on
// library-go writing tls.crt leaf-first ([leaf, CA...]). Callers verifying
// serving or client certs should assert IsCA==false on the returned certInfo.
func parseCertificate(certPEM []byte) (*certInfo, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	info := &certInfo{
		IsCA: cert.IsCA,
	}

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

// waitForSecretRegeneration waits for a secret to be recreated with new UID and populated cert data.
// oldUID is the UID of the secret before deletion; certKey is the data key to verify (e.g., "tls.crt").
// The timeout is controlled by ctx (e.g., use context.WithTimeout).
func waitForSecretRegeneration(ctx context.Context, kubeClient kubernetes.Interface, namespace, secretName, certKey string, oldUID string) error {
	lw := cache.NewListWatchFromClient(
		kubeClient.CoreV1().RESTClient(), "secrets", namespace,
		fields.OneTermEqualSelector("metadata.name", secretName))
	_, err := watchtools.UntilWithSync(ctx, lw, &corev1.Secret{}, nil,
		func(event watch.Event) (bool, error) {
			if event.Type != watch.Added && event.Type != watch.Modified {
				return false, nil
			}
			s := event.Object.(*corev1.Secret)
			if string(s.UID) == oldUID {
				return false, nil
			}
			_, ok := s.Data[certKey]
			return ok, nil
		})
	return err
}

// deleteCertificateSecret deletes a certificate secret to trigger rotation/regeneration
func deleteCertificateSecret(ctx context.Context, kubeClient kubernetes.Interface, namespace, secretName string) error {
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
