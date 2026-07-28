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
	kubeAPIServerOperatorNamespace = "openshift-kube-apiserver-operator"
	kubeAPIServerNamespace         = "openshift-kube-apiserver"
)

var _ = g.Describe("[sig-kube-apiserver][OCPFeatureGate:ConfigurablePKI][Serial][Disruptive][Suite:openshift/pkiconfig] PKI Configuration", g.Ordered, func() {
	oc := exutil.NewCLIWithoutNamespace("kube-apiserver-pki")

	var kubeClient kubernetes.Interface
	var configClient configclient.Interface

	g.BeforeAll(func(ctx context.Context) {
		kubeClient = oc.AdminKubeClient()
		configClient = oc.AdminConfigClient()

		// Register cleanup to run even if tests fail
		g.DeferCleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
			defer cancel()
			cleanupPKIConfiguration(cleanupCtx, configClient)
		})
	})

	g.It("should validate uniform PKI configurations and certificate regeneration [apigroup:config.openshift.io][Skipped:MicroShift]", func(ctx context.Context) {
		testUniformPKIConfigurations(ctx, kubeClient, configClient)
	})

	g.It("should validate mixed PKI configurations and certificate regeneration [apigroup:config.openshift.io][Skipped:MicroShift]", func(ctx context.Context) {
		testMixedPKIConfigurations(ctx, kubeClient, configClient)
	})
})

// testUniformPKIConfigurations tests uniform PKI configurations (same algorithm for all cert categories)
func testUniformPKIConfigurations(ctx context.Context, kubeClient kubernetes.Interface, configClient configclient.Interface) {
	e2e.Logf("Testing uniform PKI configurations for kube-apiserver...")

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

		// Apply the PKI configuration
		err := applyPKIConfig(ctx, configClient, tc)
		o.Expect(err).NotTo(o.HaveOccurred(), "error applying PKI config %s", tc.name)

		e2e.Logf("PKI configuration %s applied successfully", tc.name)

		// Allow the operator to observe the PKI config change before checking
		// Progressing status. PKI config changes do not trigger a full operator
		// rollout (Progressing stays False), so WaitForOperatorToRollout would
		// hang. The actual cert regeneration is triggered by secret deletion
		// below, and waitForSecretRegeneration validates the outcome.
		time.Sleep(10 * time.Second)

		e2e.Logf("Waiting for kube-apiserver operator to reconcile PKI config...")
		err = exutil.WaitForOperatorProgressingFalse(ctx, configClient, "kube-apiserver")
		o.Expect(err).NotTo(o.HaveOccurred(), "kube-apiserver operator did not reconcile PKI config %s", tc.name)
		e2e.Logf("Operator has reconciled PKI configuration")

		// Test certificate regeneration for kube-apiserver certificates
		e2e.Logf("Testing kube-apiserver certificate regeneration with %s...", tc.name)
		testKubeAPIServerCertificates(ctx, kubeClient, tc)

		e2e.Logf("Configuration %s tested successfully", tc.name)
	}

	e2e.Logf("\nAll uniform PKI configuration tests passed successfully")
}

// testMixedPKIConfigurations tests mixed PKI configurations (different algorithms per cert category)
func testMixedPKIConfigurations(ctx context.Context, kubeClient kubernetes.Interface, configClient configclient.Interface) {
	e2e.Logf("Testing mixed PKI configurations (different key types per certificate category)...")

	mixedConfigs := []mixedPKITestConfig{
		{
			name:              "RSA4096-P256-P521",
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

		// Apply the mixed PKI configuration
		err := applyMixedPKIConfig(ctx, configClient, tc)
		o.Expect(err).NotTo(o.HaveOccurred(), "error applying mixed PKI config %s", tc.name)

		e2e.Logf("Mixed PKI configuration %s applied successfully", tc.name)

		// Allow the operator to observe the PKI config change (see comment in
		// testUniformPKIConfigurations for why WaitForOperatorToRollout is not
		// used here).
		time.Sleep(10 * time.Second)

		e2e.Logf("Waiting for kube-apiserver operator to reconcile PKI config...")
		err = exutil.WaitForOperatorProgressingFalse(ctx, configClient, "kube-apiserver")
		o.Expect(err).NotTo(o.HaveOccurred(), "kube-apiserver operator did not reconcile PKI config %s", tc.name)
		e2e.Logf("Operator has reconciled PKI configuration")

		// Test certificate regeneration with mixed configuration
		e2e.Logf("Testing kube-apiserver certificate regeneration with mixed config %s...", tc.name)
		testMixedKubeAPIServerCertificates(ctx, kubeClient, tc)

		e2e.Logf("Mixed configuration %s tested successfully", tc.name)
	}

	e2e.Logf("\nAll mixed PKI configuration tests passed successfully")
}

// testKubeAPIServerCertificates tests certificate regeneration for kube-apiserver
func testKubeAPIServerCertificates(ctx context.Context, kubeClient kubernetes.Interface, tc pkiTestConfig) {
	// Test a subset of certificates to avoid excessive test time
	// Pick one from each category (signer, serving, client)
	testCerts := []operatorCertificate{
		// One signer from operator namespace
		{
			Namespace:  kubeAPIServerOperatorNamespace,
			SecretName: "aggregator-client-signer",
			CertKey:    "tls.crt",
			Category:   "signer",
		},
		// One serving cert from apiserver namespace
		{
			Namespace:  kubeAPIServerNamespace,
			SecretName: "external-loadbalancer-serving-certkey",
			CertKey:    "tls.crt",
			Category:   "serving",
		},
		// One client cert from apiserver namespace
		{
			Namespace:  kubeAPIServerNamespace,
			SecretName: "aggregator-client",
			CertKey:    "tls.crt",
			Category:   "client",
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

		// Verify the regenerated certificate matches expected config
		newCert, err := getCertificateFromSecret(ctx, kubeClient, cert.Namespace, cert.SecretName, cert.CertKey)
		o.Expect(err).NotTo(o.HaveOccurred(), "error getting regenerated certificate %s/%s", cert.Namespace, cert.SecretName)

		if cert.Category == "signer" {
			o.Expect(newCert.IsCA).To(o.BeTrue(), "signer certificate %s/%s should be a CA", cert.Namespace, cert.SecretName)
		} else {
			o.Expect(newCert.IsCA).To(o.BeFalse(), "%s certificate %s/%s should not be a CA (leaf-first PEM ordering may have changed)", cert.Category, cert.Namespace, cert.SecretName)
		}

		// Verify algorithm and key parameters
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

		// Small delay between deletions to avoid overwhelming the operators
		time.Sleep(5 * time.Second)
	}

	o.Expect(verifiedCount).To(o.BeNumerically(">", 0), "at least one certificate must be verified")
	e2e.Logf("  Configuration test completed: %d certificates verified", verifiedCount)
}

// testMixedKubeAPIServerCertificates tests certificate regeneration with mixed key configurations
func testMixedKubeAPIServerCertificates(ctx context.Context, kubeClient kubernetes.Interface, tc mixedPKITestConfig) {
	// Test one cert from each category to verify different key types
	testCerts := []struct {
		cert               operatorCertificate
		expectedAlgorithm  configv1alpha1.KeyAlgorithm
		expectedRSASize    int32
		expectedECDSACurve configv1alpha1.ECDSACurve
	}{
		{
			cert: operatorCertificate{
				Namespace:  kubeAPIServerOperatorNamespace,
				SecretName: "aggregator-client-signer",
				CertKey:    "tls.crt",
				Category:   "signer",
			},
			expectedAlgorithm:  tc.signerAlgorithm,
			expectedRSASize:    tc.signerRSASize,
			expectedECDSACurve: tc.signerECDSACurve,
		},
		{
			cert: operatorCertificate{
				Namespace:  kubeAPIServerNamespace,
				SecretName: "external-loadbalancer-serving-certkey",
				CertKey:    "tls.crt",
				Category:   "serving",
			},
			expectedAlgorithm:  tc.servingAlgorithm,
			expectedRSASize:    tc.servingRSASize,
			expectedECDSACurve: tc.servingECDSACurve,
		},
		{
			cert: operatorCertificate{
				Namespace:  kubeAPIServerNamespace,
				SecretName: "aggregator-client",
				CertKey:    "tls.crt",
				Category:   "client",
			},
			expectedAlgorithm:  tc.clientAlgorithm,
			expectedRSASize:    tc.clientRSASize,
			expectedECDSACurve: tc.clientECDSACurve,
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

		// Verify the regenerated certificate matches expected config
		newCert, err := getCertificateFromSecret(ctx, kubeClient, cert.Namespace, cert.SecretName, cert.CertKey)
		o.Expect(err).NotTo(o.HaveOccurred(), "error getting regenerated certificate %s/%s", cert.Namespace, cert.SecretName)

		if cert.Category == "signer" {
			o.Expect(newCert.IsCA).To(o.BeTrue(), "signer certificate %s/%s should be a CA", cert.Namespace, cert.SecretName)
		} else {
			o.Expect(newCert.IsCA).To(o.BeFalse(), "%s certificate %s/%s should not be a CA (leaf-first PEM ordering may have changed)", cert.Category, cert.Namespace, cert.SecretName)
		}

		// Verify algorithm and key parameters
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

		// Small delay between deletions
		time.Sleep(5 * time.Second)
	}

	o.Expect(verifiedCount).To(o.BeNumerically(">", 0), "at least one certificate must be verified")
	e2e.Logf("  Configuration test completed: %d certificates verified", verifiedCount)
}
