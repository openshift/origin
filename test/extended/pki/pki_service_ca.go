package pki

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-service-ca][OCPFeatureGate:ConfigurablePKI][Serial][Disruptive][Suite:openshift/pkiconfig] PKI Configuration", g.Ordered, func() {
	oc := exutil.NewCLIWithoutNamespace("service-ca-pki")

	var configClient configclient.Interface

	g.BeforeAll(func(ctx context.Context) {
		configClient = oc.AdminConfigClient()

		// Register cleanup to reset PKI configuration even if tests fail
		g.DeferCleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
			defer cancel()
			cleanupPKIConfiguration(cleanupCtx, configClient)
		})
	})

	g.It("should validate PKI config and regenerate signing cert [apigroup:config.openshift.io][Skipped:MicroShift]", func(ctx context.Context) {
		testServiceCAPKIConfiguration(ctx, oc)
	})
})

// testServiceCAPKIConfiguration tests the PKI configuration flow:
// 1. Verify ConfigurablePKI feature gate is enabled
// 2. Test multiple PKI configurations (RSA and ECDSA)
// 3. For each config, test CA signing cert and service cert generation
//
// NOTE: This test assumes ConfigurablePKI feature gate is already enabled via
// FEATURE_SET=TechPreviewNoUpgrade in the CI job configuration.
func testServiceCAPKIConfiguration(ctx context.Context, oc *exutil.CLI) {
	kubeClient := oc.AdminKubeClient().(*kubernetes.Clientset)
	configClient := oc.AdminConfigClient()

	// Verify PKI CRD is available (ConfigurablePKI feature gate is enabled by FEATURE_SET=TechPreviewNoUpgrade)
	e2e.Logf("Verifying PKI CRD is available...")
	err := waitForPKICRD(ctx, configClient, 30*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("PKI CRD is available")

	// Test multiple PKI configurations
	e2e.Logf("Testing multiple PKI configurations...")

	// Define test configurations
	// NOTE: RSA-8192 is NOT tested here for the following reasons:
	//
	// 1. Excessive Time: Each RSA-8192 certificate takes ~80 seconds to generate.
	//    With ~78 services in a typical cluster, a full CA rotation takes 60-90+ minutes.
	//
	// 2. Cluster Disruption: Deleting the signing-key CA triggers:
	//    - Service-CA controller restart (can't mount deleted secret)
	//    - Regeneration of ALL ~78 service certificates cluster-wide
	//    - Rolling updates of multiple operators (kube-controller-manager, kube-scheduler,
	//      kube-apiserver, monitoring, etc.) as their serving-cert secrets change
	//
	// 3. Limited Test Value: If PKI configuration works for RSA-2048, RSA-4096, and ECDSA
	//    variants, the same code path will work for RSA-8192. The key generation logic is
	//    in library-go and doesn't vary by key size.
	//
	// 4. Production Impact: This test is disruptive and should only run on dedicated test
	//    clusters. Adding RSA-8192 would multiply the disruption by 6 CA rotations instead of 5.
	//
	// RSA-8192 support can be manually verified if needed, but is excluded from automated tests.
	testConfigs := []pkiTestConfig{
		{
			name:      "RSA-4096",
			algorithm: configv1alpha1.KeyAlgorithmRSA,
			rsaSize:   4096,
		},
		{
			name:       "ECDSA-P256",
			algorithm:  configv1alpha1.KeyAlgorithmECDSA,
			ecdsaCurve: configv1alpha1.ECDSACurveP256,
		},
		{
			name:       "ECDSA-P384",
			algorithm:  configv1alpha1.KeyAlgorithmECDSA,
			ecdsaCurve: configv1alpha1.ECDSACurveP384,
		},
		{
			name:       "ECDSA-P521",
			algorithm:  configv1alpha1.KeyAlgorithmECDSA,
			ecdsaCurve: configv1alpha1.ECDSACurveP521,
		},
		// Mixed configurations: different key sizes/algorithms for signer vs serving certs
		// Stronger signer (CA), weaker serving certs - typical security practice
		{
			name:      "Mixed-RSA-4096-signer-RSA-2048-serving",
			algorithm: configv1alpha1.KeyAlgorithmRSA,
			rsaSize:   2048, // Default for serving
			signerOverride: &keyOverride{
				algorithm: configv1alpha1.KeyAlgorithmRSA,
				rsaSize:   4096,
			},
		},
		{
			name:       "Mixed-ECDSA-P384-signer-ECDSA-P256-serving",
			algorithm:  configv1alpha1.KeyAlgorithmECDSA,
			ecdsaCurve: configv1alpha1.ECDSACurveP256, // Default for serving
			signerOverride: &keyOverride{
				algorithm:  configv1alpha1.KeyAlgorithmECDSA,
				ecdsaCurve: configv1alpha1.ECDSACurveP384,
			},
		},
		// Cross-algorithm configurations: RSA and ECDSA mixed
		// Uses servingOverride to test stronger serving certs (applies to ALL serving certs)
		{
			name:       "Mixed-RSA-4096-CA-ECDSA-P384-serving",
			algorithm:  configv1alpha1.KeyAlgorithmECDSA,
			ecdsaCurve: configv1alpha1.ECDSACurveP256, // Default (not used due to overrides)
			signerOverride: &keyOverride{
				algorithm: configv1alpha1.KeyAlgorithmRSA,
				rsaSize:   4096,
			},
			servingOverride: &keyOverride{
				algorithm:  configv1alpha1.KeyAlgorithmECDSA,
				ecdsaCurve: configv1alpha1.ECDSACurveP384, // All serving certs use P384
			},
		},
		{
			name:      "Mixed-ECDSA-P384-CA-RSA-4096-serving",
			algorithm: configv1alpha1.KeyAlgorithmRSA,
			rsaSize:   2048, // Default (not used due to overrides)
			signerOverride: &keyOverride{
				algorithm:  configv1alpha1.KeyAlgorithmECDSA,
				ecdsaCurve: configv1alpha1.ECDSACurveP384,
			},
			servingOverride: &keyOverride{
				algorithm: configv1alpha1.KeyAlgorithmRSA,
				rsaSize:   4096, // All serving certs use RSA-4096
			},
		},
	}

	for _, tc := range testConfigs {
		e2e.Logf("\n=== Testing configuration: %s ===", tc.name)

		// Apply the PKI configuration
		err = applyPKIConfig(ctx, configClient, tc)
		o.Expect(err).NotTo(o.HaveOccurred(), "error applying PKI config %s", tc.name)

		// Verify the configuration was applied
		pki, err := configClient.ConfigV1alpha1().PKIs().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pki.Spec.CertificateManagement.Mode).To(o.Equal(configv1alpha1.PKICertificateManagementModeCustom))

		e2e.Logf("PKI configuration %s applied successfully", tc.name)

		// Wait for operator to reconcile the PKI config change
		// Check ClusterOperator status to ensure it's fully reconciled
		e2e.Logf("Waiting for operator to reconcile PKI config...")
		err = exutil.WaitForOperatorProgressingFalse(ctx, configClient, "service-ca")
		o.Expect(err).NotTo(o.HaveOccurred(), "service-ca operator did not reconcile PKI config %s", tc.name)
		e2e.Logf("Operator has reconciled PKI configuration")

		// Wait for controller deployment to be updated with the new PKI config
		// The operator updates the controller deployment with PKI config via command-line args
		// The controller is what generates serving certs, so it must have the new config
		e2e.Logf("Waiting for controller deployment to be updated with PKI config...")
		err = exutil.WaitForDeploymentReadyWithTimeout(oc, "service-ca", "openshift-service-ca", -1, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Controller deployment has been updated with PKI configuration")

		// Test CA signing certificate regeneration
		e2e.Logf("Testing CA signing certificate regeneration with %s...", tc.name)
		testSigningCertRegeneration(ctx, kubeClient, tc)

		// Test service certificate generation
		e2e.Logf("Testing service certificate generation with %s...", tc.name)
		testServiceCertGeneration(ctx, kubeClient, tc)

		e2e.Logf("Configuration %s tested successfully", tc.name)
	}

	e2e.Logf("\nAll PKI configuration tests passed successfully")
}

// testSigningCertRegeneration tests CA signing certificate regeneration
func testSigningCertRegeneration(ctx context.Context, kubeClient *kubernetes.Clientset, tc pkiTestConfig) {
	// Get the current UID of the signing-key secret before deletion
	signingKeySecret, err := kubeClient.CoreV1().Secrets("openshift-service-ca").Get(ctx, "signing-key", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	oldSigningKeyUID := string(signingKeySecret.UID)

	// Get the current UID of the operator serving cert before deletion (may not exist)
	var oldServingCertUID string
	servingCertSecret, err := kubeClient.CoreV1().Secrets("openshift-service-ca-operator").Get(ctx, "serving-cert", metav1.GetOptions{})
	if err == nil {
		oldServingCertUID = string(servingCertSecret.UID)
	} else if !apierrors.IsNotFound(err) {
		o.Expect(err).NotTo(o.HaveOccurred(), "error reading existing operator serving cert")
	}

	// Delete the signing certificate to trigger regeneration
	err = kubeClient.CoreV1().Secrets("openshift-service-ca").Delete(ctx, "signing-key", metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Also delete the operator's serving cert to ensure full regeneration with new PKI config
	err = kubeClient.CoreV1().Secrets("openshift-service-ca-operator").Delete(ctx, "serving-cert", metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	e2e.Logf("  Deleted CA signing certificate and operator serving cert, waiting for regeneration...")

	// Wait for regeneration (increased timeout to handle operator exhaustion after multiple reconfigurations)
	err = waitForSecretRegeneration(ctx, kubeClient, "openshift-service-ca", "signing-key", "tls.crt", oldSigningKeyUID, 10*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Verify new certificate matches expected config
	// For signing cert, use signerOverride if present, otherwise use default
	newCert, err := getCertificateFromSecret(ctx, kubeClient, "openshift-service-ca", "signing-key", "tls.crt")
	o.Expect(err).NotTo(o.HaveOccurred())

	// Determine expected values for signing cert (CA)
	expectedAlgo := tc.algorithm
	expectedRSASize := tc.rsaSize
	expectedCurve := tc.ecdsaCurve
	if tc.signerOverride != nil {
		expectedAlgo = tc.signerOverride.algorithm
		expectedRSASize = tc.signerOverride.rsaSize
		expectedCurve = tc.signerOverride.ecdsaCurve
	}

	// Verify algorithm and key parameters
	if expectedAlgo == configv1alpha1.KeyAlgorithmRSA {
		o.Expect(newCert.Algorithm).To(o.Equal("RSA"))
		o.Expect(int32(newCert.KeySize)).To(o.Equal(expectedRSASize))
		e2e.Logf("  CA signing cert: RSA-%d", newCert.KeySize)
	} else if expectedAlgo == configv1alpha1.KeyAlgorithmECDSA {
		o.Expect(newCert.Algorithm).To(o.Equal("ECDSA"))
		expectedCurveStr := string(expectedCurve)
		o.Expect(newCert.Curve).To(o.Equal(expectedCurveStr))
		e2e.Logf("  CA signing cert: ECDSA-%s", newCert.Curve)
	}

	// Wait for controller to be ready after signing-key regeneration
	// The controller pod will restart when signing-key is deleted/recreated
	e2e.Logf("  Waiting for controller to be ready...")
	_, err = exutil.WaitForPods(kubeClient.CoreV1().Pods("openshift-service-ca"), exutil.ParseLabelsOrDie("app=service-ca"), exutil.CheckPodIsReady, 1, 5*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Wait for operator's serving cert to be regenerated (increased timeout for operator exhaustion)
	err = waitForSecretRegeneration(ctx, kubeClient, "openshift-service-ca-operator", "serving-cert", "tls.crt", oldServingCertUID, 15*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Verify operator's serving cert also uses the new PKI config
	operatorCert, err := getCertificateFromSecret(ctx, kubeClient, "openshift-service-ca-operator", "serving-cert", "tls.crt")
	o.Expect(err).NotTo(o.HaveOccurred())

	// Determine expected values for operator serving cert
	// Use servingOverride if present, otherwise use default
	expectedServingAlgo := tc.algorithm
	expectedServingRSASize := tc.rsaSize
	expectedServingCurve := tc.ecdsaCurve
	if tc.servingOverride != nil {
		expectedServingAlgo = tc.servingOverride.algorithm
		expectedServingRSASize = tc.servingOverride.rsaSize
		expectedServingCurve = tc.servingOverride.ecdsaCurve
	}

	if expectedServingAlgo == configv1alpha1.KeyAlgorithmRSA {
		o.Expect(operatorCert.Algorithm).To(o.Equal("RSA"))
		o.Expect(int32(operatorCert.KeySize)).To(o.Equal(expectedServingRSASize))
		e2e.Logf("  Operator serving cert: RSA-%d", operatorCert.KeySize)
	} else if expectedServingAlgo == configv1alpha1.KeyAlgorithmECDSA {
		o.Expect(operatorCert.Algorithm).To(o.Equal("ECDSA"))
		expectedCurveStr := string(expectedServingCurve)
		o.Expect(operatorCert.Curve).To(o.Equal(expectedCurveStr))
		e2e.Logf("  Operator serving cert: ECDSA-%s", operatorCert.Curve)
	}
}

// testServiceCertGeneration tests service certificate generation
func testServiceCertGeneration(ctx context.Context, kubeClient *kubernetes.Clientset, tc pkiTestConfig) {
	// Create a unique test namespace (timestamp suffix to avoid conflicts with previous runs)
	// Use hash for long names to stay under 63 character Kubernetes limit
	configName := strings.ToLower(tc.name)
	if len(configName) > 30 {
		// For long names, use first 20 chars + hash of full name
		// This ensures uniqueness while keeping under the limit
		hash := fmt.Sprintf("%x", time.Now().UnixNano()%0xFFFFFF) // 6 hex chars
		configName = configName[:20] + "-" + hash
	}
	testNS := fmt.Sprintf("test-pki-%s-%d", configName, time.Now().Unix())

	// Create the namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNS,
		},
	}
	_, err := kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Cleanup namespace at the end
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
		err := kubeClient.CoreV1().Namespaces().Delete(cleanupCtx, testNS, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			e2e.Logf("Warning: failed to delete test namespace %s: %v", testNS, err)
		}
	}()

	// Create a test service with serving-cert annotation
	testSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: testNS,
			Annotations: map[string]string{
				"service.beta.openshift.io/serving-cert-secret-name": "test-service-cert",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "https",
					Port:     443,
					Protocol: corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": "test",
			},
		},
	}

	_, err = kubeClient.CoreV1().Services(testNS).Create(ctx, testSvc, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("  Created test service in namespace %s", testNS)

	// Wait for the service certificate to be generated
	e2e.Logf("  Waiting for service certificate to be generated...")
	err = waitForSecretRegeneration(ctx, kubeClient, testNS, "test-service-cert", "tls.crt", "", 3*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Verify the service certificate
	serviceCert, err := getCertificateFromSecret(ctx, kubeClient, testNS, "test-service-cert", "tls.crt")
	o.Expect(err).NotTo(o.HaveOccurred())

	// Verify algorithm and key parameters match expected config
	// Use servingOverride if present, otherwise use default
	expectedServingAlgo := tc.algorithm
	expectedServingRSASize := tc.rsaSize
	expectedServingCurve := tc.ecdsaCurve
	if tc.servingOverride != nil {
		expectedServingAlgo = tc.servingOverride.algorithm
		expectedServingRSASize = tc.servingOverride.rsaSize
		expectedServingCurve = tc.servingOverride.ecdsaCurve
	}

	if expectedServingAlgo == configv1alpha1.KeyAlgorithmRSA {
		o.Expect(serviceCert.Algorithm).To(o.Equal("RSA"))
		o.Expect(int32(serviceCert.KeySize)).To(o.Equal(expectedServingRSASize))
		e2e.Logf("  Service cert: RSA-%d", serviceCert.KeySize)
	} else if expectedServingAlgo == configv1alpha1.KeyAlgorithmECDSA {
		o.Expect(serviceCert.Algorithm).To(o.Equal("ECDSA"))
		expectedCurveStr := string(expectedServingCurve)
		o.Expect(serviceCert.Curve).To(o.Equal(expectedCurveStr))
		e2e.Logf("  Service cert: ECDSA-%s", serviceCert.Curve)
	}
}
