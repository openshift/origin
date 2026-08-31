package pki

import (
	"context"
	"fmt"
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
	kubeControllerManagerOperatorNamespace = "openshift-kube-controller-manager-operator"
	machineConfigOperatorNamespace         = "openshift-machine-config-operator"
)

// operatorPKIScenario describes an operator whose PKI-managed certificates are
// exercised by the shared uniform and mixed PKI configuration tests. Operators
// differ only in their namespaces, cluster-operator name, and the set of
// certificate secrets they manage, so the test logic is fully shared.
type operatorPKIScenario struct {
	sigTag       string // Ginkgo sig label, e.g. "sig-mco"
	cliName      string // CLI/namespace prefix, e.g. "machine-config-operator-pki"
	displayName  string // human-readable operator name used in log output
	operatorName string // cluster-operator name used for the rollout wait

	// uniformCerts are verified for uniform configs (one algorithm everywhere).
	uniformCerts []operatorCertificate
	// mixedConfig is the mixed PKI config applied for the mixed test.
	mixedConfig mixedPKITestConfig
	// mixedCerts are verified for the mixed config; each cert is checked against
	// the profile matching its Category (signer/serving/client).
	mixedCerts []operatorCertificate
}

var operatorPKIScenarios = []operatorPKIScenario{
	{
		sigTag:       "sig-kube-controller-manager",
		cliName:      "kube-controller-manager-pki",
		displayName:  "kube-controller-manager",
		operatorName: "kube-controller-manager",
		// The kube-controller-manager operator manages only the CSR signer chain
		// (no serving/client certs), so only signers are tested. Both secrets are
		// managed by the cert rotation controller in the operator namespace.
		// Test csr-signer (child) before csr-signer-signer (parent CA) to avoid
		// cascade: deleting the parent CA triggers automatic re-signing of the
		// child, which may reuse the existing key pair rather than generating a
		// new one from the PKI profile.
		uniformCerts: []operatorCertificate{
			{Namespace: kubeControllerManagerOperatorNamespace, SecretName: "csr-signer", CertKey: "tls.crt", Category: "signer"},
			{Namespace: kubeControllerManagerOperatorNamespace, SecretName: "csr-signer-signer", CertKey: "tls.crt", Category: "signer"},
		},
		// KCM exposes no serving/client cert secrets that follow the PKI profile,
		// so the mixed config configures only the signer profile.
		mixedConfig: mixedPKITestConfig{
			name:            "RSA4096-signers",
			signerAlgorithm: configv1alpha1.KeyAlgorithmRSA,
			signerRSASize:   4096,
		},
		mixedCerts: []operatorCertificate{
			{Namespace: kubeControllerManagerOperatorNamespace, SecretName: "csr-signer", CertKey: "tls.crt", Category: "signer"},
			{Namespace: kubeControllerManagerOperatorNamespace, SecretName: "csr-signer-signer", CertKey: "tls.crt", Category: "signer"},
		},
	},
	{
		sigTag:       "sig-mco",
		cliName:      "machine-config-operator-pki",
		displayName:  "machine-config-operator",
		operatorName: "machine-config",
		// Test the serving cert (child) before the CA (parent) to avoid cascade:
		// deleting the CA triggers automatic re-signing of the serving cert,
		// which may reuse the existing key pair rather than generating a new one
		// from the PKI profile.
		uniformCerts: []operatorCertificate{
			{Namespace: machineConfigOperatorNamespace, SecretName: "machine-config-server-tls", CertKey: "tls.crt", Category: "serving"},
			{Namespace: machineConfigOperatorNamespace, SecretName: "machine-config-server-ca", CertKey: "tls.crt", Category: "signer"},
		},
		mixedConfig: mixedPKITestConfig{
			name:              "RSA4096-signer-P256-serving",
			signerAlgorithm:   configv1alpha1.KeyAlgorithmRSA,
			signerRSASize:     4096,
			servingAlgorithm:  configv1alpha1.KeyAlgorithmECDSA,
			servingECDSACurve: configv1alpha1.ECDSACurveP256,
		},
		mixedCerts: []operatorCertificate{
			{Namespace: machineConfigOperatorNamespace, SecretName: "machine-config-server-tls", CertKey: "tls.crt", Category: "serving"},
			{Namespace: machineConfigOperatorNamespace, SecretName: "machine-config-server-ca", CertKey: "tls.crt", Category: "signer"},
		},
	},
}

var _ = func() bool {
	for i := range operatorPKIScenarios {
		registerOperatorPKITests(operatorPKIScenarios[i])
	}
	return true
}()

func registerOperatorPKITests(scenario operatorPKIScenario) {
	g.Describe(fmt.Sprintf("[%s][OCPFeatureGate:ConfigurablePKI][Serial][Disruptive][Suite:openshift/pkiconfig] PKI Configuration", scenario.sigTag), g.Ordered, func() {
		oc := exutil.NewCLIWithoutNamespace(scenario.cliName)

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

		g.It(fmt.Sprintf("should validate uniform PKI configurations and certificate regeneration for %s [apigroup:config.openshift.io][Skipped:MicroShift]", scenario.displayName), func(ctx context.Context) {
			testUniformOperatorPKIConfigurations(ctx, kubeClient, configClient, scenario)
		})

		g.It(fmt.Sprintf("should validate mixed PKI configurations and certificate regeneration for %s [apigroup:config.openshift.io][Skipped:MicroShift]", scenario.displayName), func(ctx context.Context) {
			testMixedOperatorPKIConfigurations(ctx, kubeClient, configClient, scenario)
		})
	})
}

func testUniformOperatorPKIConfigurations(ctx context.Context, kubeClient kubernetes.Interface, configClient configclient.Interface, scenario operatorPKIScenario) {
	e2e.Logf("Testing uniform PKI configurations for %s...", scenario.displayName)

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

		// Allow the operator to observe the PKI config change before checking
		// Progressing status. PKI config changes do not trigger a full operator
		// rollout (Progressing stays False), so WaitForOperatorToRollout would
		// hang. The actual cert regeneration is triggered by secret deletion
		// below, and waitForSecretRegeneration validates the outcome.
		time.Sleep(10 * time.Second)

		e2e.Logf("Waiting for %s operator to reconcile PKI config...", scenario.displayName)
		err = exutil.WaitForOperatorProgressingFalse(ctx, configClient, scenario.operatorName)
		o.Expect(err).NotTo(o.HaveOccurred(), "%s operator did not reconcile PKI config %s", scenario.operatorName, tc.name)
		e2e.Logf("Operator has reconciled PKI configuration")

		e2e.Logf("Testing %s certificate regeneration with %s...", scenario.displayName, tc.name)
		testOperatorCertificates(ctx, kubeClient, scenario.uniformCerts, func(operatorCertificate) (configv1alpha1.KeyAlgorithm, int32, configv1alpha1.ECDSACurve) {
			return tc.algorithm, tc.rsaSize, tc.ecdsaCurve
		})

		e2e.Logf("Configuration %s tested successfully", tc.name)
	}

	e2e.Logf("\nAll uniform PKI configuration tests passed successfully")
}

func testMixedOperatorPKIConfigurations(ctx context.Context, kubeClient kubernetes.Interface, configClient configclient.Interface, scenario operatorPKIScenario) {
	e2e.Logf("Testing mixed PKI configurations for %s...", scenario.displayName)

	tc := scenario.mixedConfig
	e2e.Logf("\n=== Testing mixed configuration: %s ===", tc.name)

	err := applyMixedPKIConfig(ctx, configClient, tc)
	o.Expect(err).NotTo(o.HaveOccurred(), "error applying mixed PKI config %s", tc.name)

	e2e.Logf("Mixed PKI configuration %s applied successfully", tc.name)

	// Allow the operator to observe the PKI config change (see comment in
	// testUniformOperatorPKIConfigurations for why WaitForOperatorToRollout is
	// not used here).
	time.Sleep(10 * time.Second)

	e2e.Logf("Waiting for %s operator to reconcile PKI config...", scenario.displayName)
	err = exutil.WaitForOperatorProgressingFalse(ctx, configClient, scenario.operatorName)
	o.Expect(err).NotTo(o.HaveOccurred(), "%s operator did not reconcile mixed PKI config %s", scenario.operatorName, tc.name)
	e2e.Logf("Operator has reconciled PKI configuration")

	e2e.Logf("Testing %s certificate regeneration with mixed config %s...", scenario.displayName, tc.name)
	testOperatorCertificates(ctx, kubeClient, scenario.mixedCerts, func(cert operatorCertificate) (configv1alpha1.KeyAlgorithm, int32, configv1alpha1.ECDSACurve) {
		return expectedKeyConfigForCategory(tc, cert.Category)
	})

	e2e.Logf("Mixed configuration %s tested successfully", tc.name)
	e2e.Logf("\nAll mixed PKI configuration tests passed successfully")
}

// testOperatorCertificates deletes each certificate secret, waits for it to be
// regenerated, and verifies the new certificate's CA flag and key config.
// expectedFor returns the expected key algorithm/parameters for a given cert.
func testOperatorCertificates(ctx context.Context, kubeClient kubernetes.Interface, certs []operatorCertificate, expectedFor func(operatorCertificate) (configv1alpha1.KeyAlgorithm, int32, configv1alpha1.ECDSACurve)) {
	o.Expect(certs).NotTo(o.BeEmpty(), "certificate list must not be empty")

	for _, cert := range certs {
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

		verifyCertIsCA(newCert, cert.Category, cert.Namespace, cert.SecretName)
		algorithm, rsaSize, ecdsaCurve := expectedFor(cert)
		verifyCertKeyConfig(newCert, algorithm, rsaSize, ecdsaCurve, cert.Category, cert.Namespace, cert.SecretName)

		time.Sleep(5 * time.Second)
	}

	e2e.Logf("  Configuration test completed: %d certificates verified", len(certs))
}

// expectedKeyConfigForCategory returns the key algorithm/parameters a certificate
// of the given category should follow under a mixed PKI config.
func expectedKeyConfigForCategory(tc mixedPKITestConfig, category string) (configv1alpha1.KeyAlgorithm, int32, configv1alpha1.ECDSACurve) {
	switch category {
	case "serving":
		return tc.servingAlgorithm, tc.servingRSASize, tc.servingECDSACurve
	case "client":
		return tc.clientAlgorithm, tc.clientRSASize, tc.clientECDSACurve
	default: // signer
		return tc.signerAlgorithm, tc.signerRSASize, tc.signerECDSACurve
	}
}
