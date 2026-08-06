package pki

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	machineConfigOperatorNamespace = "openshift-machine-config-operator"
)

var _ = g.Describe("[sig-mco][OCPFeatureGate:ConfigurablePKI][Serial][Disruptive][Suite:openshift/pkiconfig] PKI Configuration", g.Ordered, func() {
	oc := exutil.NewCLIWithoutNamespace("machine-config-operator-pki")

	var kubeClient kubernetes.Interface
	var configClient configclient.Interface

	g.BeforeAll(func(ctx context.Context) {
		kubeClient = oc.AdminKubeClient()
		configClient = oc.AdminConfigClient()

		g.DeferCleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
			defer cancel()
			cleanupPKIConfiguration(cleanupCtx, configClient)
		})
	})

	g.It("should validate uniform PKI configurations and certificate regeneration for machine-config-operator [apigroup:config.openshift.io][Skipped:MicroShift]", func(ctx context.Context) {
		testUniformMCOPKIConfigurations(ctx, kubeClient, configClient)
	})

	g.It("should validate mixed PKI configurations and certificate regeneration for machine-config-operator [apigroup:config.openshift.io][Skipped:MicroShift]", func(ctx context.Context) {
		testMixedMCOPKIConfigurations(ctx, kubeClient, configClient)
	})
})

func testUniformMCOPKIConfigurations(ctx context.Context, kubeClient kubernetes.Interface, configClient configclient.Interface) {
	e2e.Logf("Testing uniform PKI configurations for machine-config-operator...")

	testConfigs := []pkiTestConfig{
		{
			name:      "RSA-4096",
			algorithm: configv1alpha1.KeyAlgorithmRSA,
			rsaSize:   4096,
		},
		{
			name:       "ECDSA-P384",
			algorithm:  configv1alpha1.KeyAlgorithmECDSA,
			ecdsaCurve: configv1alpha1.ECDSACurveP384,
		},
	}

	for _, tc := range testConfigs {
		e2e.Logf("\n=== Testing configuration: %s ===", tc.name)

		err := applyPKIConfig(ctx, configClient, tc)
		o.Expect(err).NotTo(o.HaveOccurred(), "error applying PKI config %s", tc.name)

		e2e.Logf("PKI configuration %s applied successfully", tc.name)

		time.Sleep(10 * time.Second)

		e2e.Logf("Waiting for machine-config operator to reconcile PKI config...")
		err = exutil.WaitForOperatorProgressingFalse(ctx, configClient, "machine-config")
		o.Expect(err).NotTo(o.HaveOccurred(), "machine-config operator did not reconcile PKI config %s", tc.name)
		e2e.Logf("Operator has reconciled PKI configuration")

		e2e.Logf("Testing machine-config-operator certificate regeneration with %s...", tc.name)
		testMCOCertificates(ctx, kubeClient, tc)

		e2e.Logf("Configuration %s tested successfully", tc.name)
	}

	e2e.Logf("\nAll uniform PKI configuration tests passed successfully")
}

func testMixedMCOPKIConfigurations(ctx context.Context, kubeClient kubernetes.Interface, configClient configclient.Interface) {
	e2e.Logf("Testing mixed PKI configurations for machine-config-operator...")

	mixedConfigs := []mixedPKITestConfig{
		{
			name:              "RSA4096-signer-P256-serving",
			signerAlgorithm:   configv1alpha1.KeyAlgorithmRSA,
			signerRSASize:     4096,
			servingAlgorithm:  configv1alpha1.KeyAlgorithmECDSA,
			servingECDSACurve: configv1alpha1.ECDSACurveP256,
			clientAlgorithm:   configv1alpha1.KeyAlgorithmECDSA,
			clientECDSACurve:  configv1alpha1.ECDSACurveP521,
		},
	}

	for _, tc := range mixedConfigs {
		e2e.Logf("\n=== Testing mixed configuration: %s ===", tc.name)

		err := applyMixedPKIConfig(ctx, configClient, tc)
		o.Expect(err).NotTo(o.HaveOccurred(), "error applying mixed PKI config %s", tc.name)

		e2e.Logf("Mixed PKI configuration %s applied successfully", tc.name)

		time.Sleep(10 * time.Second)

		e2e.Logf("Waiting for machine-config operator to reconcile PKI config...")
		err = exutil.WaitForOperatorProgressingFalse(ctx, configClient, "machine-config")
		o.Expect(err).NotTo(o.HaveOccurred(), "machine-config operator did not reconcile PKI config %s", tc.name)
		e2e.Logf("Operator has reconciled PKI configuration")

		e2e.Logf("Testing machine-config-operator certificate regeneration with mixed config %s...", tc.name)
		testMixedMCOCertificates(ctx, kubeClient, tc)

		e2e.Logf("Mixed configuration %s tested successfully", tc.name)
	}

	e2e.Logf("\nAll mixed PKI configuration tests passed successfully")
}

func testMCOCertificates(ctx context.Context, kubeClient kubernetes.Interface, tc pkiTestConfig) {
	// Test the serving cert (child) before the CA (parent) to avoid cascade:
	// deleting the CA triggers automatic re-signing of the serving cert,
	// which may reuse the existing key pair rather than generating a new one
	// from the PKI profile.
	testCerts := []operatorCertificate{
		{
			Namespace:  machineConfigOperatorNamespace,
			SecretName: "machine-config-server-tls",
			CertKey:    "tls.crt",
			Category:   "serving",
		},
		{
			Namespace:  machineConfigOperatorNamespace,
			SecretName: "machine-config-server-ca",
			CertKey:    "tls.crt",
			Category:   "signer",
		},
	}

	verifiedCount := 0
	for _, cert := range testCerts {
		e2e.Logf("  Testing %s certificate: %s/%s", cert.Category, cert.Namespace, cert.SecretName)

		oldSecret, err := kubeClient.CoreV1().Secrets(cert.Namespace).Get(ctx, cert.SecretName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "certificate %s/%s must exist before deletion", cert.Namespace, cert.SecretName)
		oldUID := string(oldSecret.UID)

		err = deleteCertificateSecret(ctx, kubeClient, cert.Namespace, cert.SecretName)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete certificate %s/%s", cert.Namespace, cert.SecretName)
		e2e.Logf("    Certificate deleted")

		e2e.Logf("    Waiting for certificate regeneration...")

		regenCtx, regenCancel := context.WithTimeout(ctx, 3*time.Minute)
		err = waitForSecretRegeneration(regenCtx, kubeClient, cert.Namespace, cert.SecretName, cert.CertKey, oldUID)
		regenCancel()
		o.Expect(err).NotTo(o.HaveOccurred(), "error waiting for certificate %s/%s regeneration", cert.Namespace, cert.SecretName)
		e2e.Logf("    Certificate regenerated")

		newCert, err := getCertificateFromSecret(ctx, kubeClient, cert.Namespace, cert.SecretName, cert.CertKey)
		o.Expect(err).NotTo(o.HaveOccurred(), "error getting regenerated certificate %s/%s", cert.Namespace, cert.SecretName)

		if cert.Category == "signer" {
			o.Expect(newCert.IsCA).To(o.BeTrue(), "signer certificate %s/%s should be a CA", cert.Namespace, cert.SecretName)
		} else {
			o.Expect(newCert.IsCA).To(o.BeFalse(), "%s certificate %s/%s should not be a CA (leaf-first PEM ordering may have changed)", cert.Category, cert.Namespace, cert.SecretName)
		}

		if tc.algorithm == configv1alpha1.KeyAlgorithmRSA {
			o.Expect(newCert.Algorithm).To(o.Equal("RSA"), "expected RSA algorithm for %s/%s", cert.Namespace, cert.SecretName)
			o.Expect(int32(newCert.KeySize)).To(o.Equal(tc.rsaSize), "expected RSA key size %d for %s/%s", tc.rsaSize, cert.Namespace, cert.SecretName)
			e2e.Logf("    Certificate verified: RSA-%d", newCert.KeySize)
		} else if tc.algorithm == configv1alpha1.KeyAlgorithmECDSA {
			o.Expect(newCert.Algorithm).To(o.Equal("ECDSA"), "expected ECDSA algorithm for %s/%s", cert.Namespace, cert.SecretName)
			expectedCurve := string(tc.ecdsaCurve)
			o.Expect(newCert.Curve).To(o.Equal(expectedCurve), "expected ECDSA curve %s for %s/%s", expectedCurve, cert.Namespace, cert.SecretName)
			e2e.Logf("    Certificate verified: ECDSA-%s", newCert.Curve)
		}

		verifiedCount++

		time.Sleep(5 * time.Second)
	}

	o.Expect(verifiedCount).To(o.BeNumerically(">", 0), "at least one certificate must be verified")
	e2e.Logf("  Configuration test completed: %d certificates verified", verifiedCount)
}

func testMixedMCOCertificates(ctx context.Context, kubeClient kubernetes.Interface, tc mixedPKITestConfig) {
	testCerts := []struct {
		cert               operatorCertificate
		expectedAlgorithm  configv1alpha1.KeyAlgorithm
		expectedRSASize    int32
		expectedECDSACurve configv1alpha1.ECDSACurve
	}{
		{
			cert: operatorCertificate{
				Namespace:  machineConfigOperatorNamespace,
				SecretName: "machine-config-server-tls",
				CertKey:    "tls.crt",
				Category:   "serving",
			},
			expectedAlgorithm:  tc.servingAlgorithm,
			expectedRSASize:    tc.servingRSASize,
			expectedECDSACurve: tc.servingECDSACurve,
		},
		{
			cert: operatorCertificate{
				Namespace:  machineConfigOperatorNamespace,
				SecretName: "machine-config-server-ca",
				CertKey:    "tls.crt",
				Category:   "signer",
			},
			expectedAlgorithm:  tc.signerAlgorithm,
			expectedRSASize:    tc.signerRSASize,
			expectedECDSACurve: tc.signerECDSACurve,
		},
	}

	verifiedCount := 0
	for _, testCase := range testCerts {
		cert := testCase.cert
		e2e.Logf("  Testing %s certificate: %s/%s", cert.Category, cert.Namespace, cert.SecretName)

		oldSecret, err := kubeClient.CoreV1().Secrets(cert.Namespace).Get(ctx, cert.SecretName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "certificate %s/%s must exist before deletion", cert.Namespace, cert.SecretName)
		oldUID := string(oldSecret.UID)

		err = deleteCertificateSecret(ctx, kubeClient, cert.Namespace, cert.SecretName)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete certificate %s/%s", cert.Namespace, cert.SecretName)
		e2e.Logf("    Certificate deleted")

		e2e.Logf("    Waiting for certificate regeneration...")

		regenCtx, regenCancel := context.WithTimeout(ctx, 3*time.Minute)
		err = waitForSecretRegeneration(regenCtx, kubeClient, cert.Namespace, cert.SecretName, cert.CertKey, oldUID)
		regenCancel()
		o.Expect(err).NotTo(o.HaveOccurred(), "error waiting for certificate %s/%s regeneration", cert.Namespace, cert.SecretName)
		e2e.Logf("    Certificate regenerated")

		newCert, err := getCertificateFromSecret(ctx, kubeClient, cert.Namespace, cert.SecretName, cert.CertKey)
		o.Expect(err).NotTo(o.HaveOccurred(), "error getting regenerated certificate %s/%s", cert.Namespace, cert.SecretName)

		if cert.Category == "signer" {
			o.Expect(newCert.IsCA).To(o.BeTrue(), "signer certificate %s/%s should be a CA", cert.Namespace, cert.SecretName)
		} else {
			o.Expect(newCert.IsCA).To(o.BeFalse(), "%s certificate %s/%s should not be a CA (leaf-first PEM ordering may have changed)", cert.Category, cert.Namespace, cert.SecretName)
		}

		if testCase.expectedAlgorithm == configv1alpha1.KeyAlgorithmRSA {
			o.Expect(newCert.Algorithm).To(o.Equal("RSA"), "expected RSA algorithm for %s certificate %s/%s", cert.Category, cert.Namespace, cert.SecretName)
			o.Expect(int32(newCert.KeySize)).To(o.Equal(testCase.expectedRSASize), "expected RSA key size %d for %s certificate %s/%s", testCase.expectedRSASize, cert.Category, cert.Namespace, cert.SecretName)
			e2e.Logf("    %s certificate verified: RSA-%d", cert.Category, newCert.KeySize)
		} else if testCase.expectedAlgorithm == configv1alpha1.KeyAlgorithmECDSA {
			o.Expect(newCert.Algorithm).To(o.Equal("ECDSA"), "expected ECDSA algorithm for %s certificate %s/%s", cert.Category, cert.Namespace, cert.SecretName)
			expectedCurve := string(testCase.expectedECDSACurve)
			o.Expect(newCert.Curve).To(o.Equal(expectedCurve), "expected ECDSA curve %s for %s certificate %s/%s", expectedCurve, cert.Category, cert.Namespace, cert.SecretName)
			e2e.Logf("    %s certificate verified: ECDSA-%s", cert.Category, newCert.Curve)
		}

		verifiedCount++

		time.Sleep(5 * time.Second)
	}

	o.Expect(verifiedCount).To(o.BeNumerically(">", 0), "at least one certificate must be verified")
	e2e.Logf("  Configuration test completed: %d certificates verified", verifiedCount)
}
