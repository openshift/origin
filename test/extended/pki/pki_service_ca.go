package pki

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
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

var _ = g.Describe("[sig-service-ca] Service CA serving certificate management", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("service-ca-serving-cert")

	g.It("[OTP][OCP-10873] should access app through secure service and regenerate service serving certs when about to expire [Slow][ConnectedOnly][Critical]", ote.Informing(), func() {
		var (
			filename     = "aosqe-pod-for-ping.json"
			podName      = "hello-pod"
			caseID       = "ocp10873"
			stepExecTime time.Time
			tmpdir       = "/tmp/ocp10873-" + time.Now().Format("20060102-150405")
		)

		defer func() {
			os.RemoveAll(tmpdir)
		}()

		err := os.MkdirAll(tmpdir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("1) Create new project for the test case")
		oc.SetupProject()
		testNamespace := oc.Namespace()

		compat_otp.By("2) Apply appropriate pod security labels to the new project")
		_, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", testNamespace, "security.openshift.io/scc.podSecurityLabelSync=false", "--overwrite").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", testNamespace, "pod-security.kubernetes.io/warn=privileged", "--overwrite").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", testNamespace, "pod-security.kubernetes.io/audit=privileged", "--overwrite").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().WithoutNamespace().Run("label").Args("ns", testNamespace, "pod-security.kubernetes.io/enforce=privileged", "--overwrite").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("3) Add SCC privileged to the project")
		err = oc.AsAdmin().WithoutNamespace().Run("adm").Args("policy", "add-scc-to-group", "privileged", "system:serviceaccounts:"+testNamespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By("4) Create a service with serving-cert annotation")
		template := exutil.FixturePath("testdata", "apiserver", caseID+"-svc.json")
		svcErr := oc.Run("create").Args("-f", template).Execute()
		o.Expect(svcErr).NotTo(o.HaveOccurred())

		stepExecTime = time.Now()

		compat_otp.By("5) Create a nginx webserver app with deployment")
		template = exutil.FixturePath("testdata", "apiserver", caseID+"-dc.yaml")
		dcErr := oc.Run("create").Args("-f", template).Execute()
		o.Expect(dcErr).NotTo(o.HaveOccurred())

		podsOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", testNamespace, "-l", "name=web-server-rc", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		pods := strings.Fields(podsOutput)
		o.Expect(pods).NotTo(o.BeEmpty())
		appPodName := pods[0]
		compat_otp.AssertPodToBeReady(oc, appPodName, testNamespace)
		cmName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("configmaps", "nginx-config", "-n", testNamespace, "-o=jsonpath={.metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cmName).ShouldNot(o.BeEmpty(), "The ConfigMap 'nginx-config' name should not be empty")

		compat_otp.By(fmt.Sprintf("6.1) Create pod with resource file %s", filename))
		template = exutil.FixturePath("testdata", "apiserver", filename)
		err = oc.Run("create").Args("-f", template).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		compat_otp.By(fmt.Sprintf("6.2) Wait for pod with name %s to be ready", podName))
		compat_otp.AssertPodToBeReady(oc, podName, testNamespace)

		url := fmt.Sprintf("https://hello.%s.svc:443", testNamespace)
		execCmd := fmt.Sprintf("curl --cacert /var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt %s", url)
		curlCmdOutput, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", testNamespace, podName, "--", "/bin/sh", "-c", execCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(curlCmdOutput).Should(o.ContainSubstring("Hello-OpenShift"))

		compat_otp.By("7) Extract the cert and key from secret ssl-key")
		err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("-n", testNamespace, "secret/ssl-key", "--to", tmpdir).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		tlsCrtFile := filepath.Join(tmpdir, "tls.crt")
		tlsCrt, err := os.ReadFile(tlsCrtFile)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tlsCrt).ShouldNot(o.BeEmpty())

		// Set the new expiry(1 hour + 1 minute) after the time of the secret ssl-key was created
		compat_otp.By("8) Set the new expiry annotations to the secret ssl-key")
		tlsCrtCreation, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", "ssl-key", "-n", testNamespace, "-o=jsonpath={.metadata.creationTimestamp}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(tlsCrtCreation).ShouldNot(o.BeEmpty())
		e2e.Logf("created time:%s", tlsCrtCreation)
		tlsCrtCreationTime, err := time.Parse(time.RFC3339, tlsCrtCreation)
		o.Expect(err).NotTo(o.HaveOccurred())
		newExpiry := tlsCrtCreationTime.Add(time.Since(stepExecTime) + 1*time.Hour + 60*time.Second)
		newExpiryStr := fmt.Sprintf(`"%s"`, newExpiry.Format(time.RFC3339))
		e2e.Logf("The new expiry of the secret ssl-key is %s", newExpiryStr)

		annotationPatch := fmt.Sprintf(`{"metadata":{"annotations": {"service.alpha.openshift.io/expiry": %s, "service.beta.openshift.io/expiry": %s}}}`, newExpiryStr, newExpiryStr)
		errPatch := oc.AsAdmin().WithoutNamespace().Run("patch").Args("secret", "ssl-key", "-n", testNamespace, "--type=merge", "-p", annotationPatch).Execute()
		o.Expect(errPatch).NotTo(o.HaveOccurred())

		compat_otp.By("9) Check secret ssl-key regenerates when expiry time drops below 1h")
		o.Eventually(func() bool {
			err = oc.AsAdmin().WithoutNamespace().Run("extract").Args("-n", testNamespace, "secret/ssl-key", "--to", tmpdir, "--confirm=true").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			tlsCrt1, err := os.ReadFile(tlsCrtFile)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(tlsCrt1).ShouldNot(o.BeEmpty())
			if !bytes.Equal(tlsCrt, tlsCrt1) {
				e2e.Logf("When the expiry time has less than 1h left, the cert has been regenerated")
				return true
			}
			e2e.Logf("When the expiry time has more than 1h left, the cert will not regenerate")
			return false
		}, "25m", "60s").Should(o.Equal(true),
			"Failed to regenerate the new secret ssl-key when the expiry time drops below 1h")

		compat_otp.By(fmt.Sprintf("10) Verify web app is still accessible in pod %s with the regenerated secret ssl-key", podName))
		compat_otp.AssertPodToBeReady(oc, podName, testNamespace)
		curlCmdOutput, err = oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", testNamespace, podName, "--", "/bin/sh", "-c", execCmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(curlCmdOutput).Should(o.ContainSubstring("Hello-OpenShift"))
	})
})

var _ = g.Describe("[sig-service-ca] Service CA certificate validation", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("service-ca-validation")

	g.It("[OTP][OCP-56934] should verify no duplicate CA serial number errors in openshift-apiserver metrics [PstChkUpgrade][apigroup:config.openshift.io]", ote.Informing(), func() {
		g.By("1) Check if it's a MicroShift cluster")
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Not supported on MicroShift - no openshift-apiserver")
		}

		g.By("2) Check if it's a proxy cluster")
		httpProxy, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("proxy/cluster", "-o", "jsonpath={.spec.httpProxy}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		httpsProxy, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("proxy/cluster", "-o", "jsonpath={.spec.httpsProxy}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		if strings.Contains(httpProxy, "http") || strings.Contains(httpsProxy, "https") {
			g.Skip("Skip for proxy platform")
		}

		g.By("3) Get openshift-apiserver pod and pod IP")
		podName, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-apiserver", "pod", "--field-selector=status.phase=Running", "-o", "jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podName).NotTo(o.BeEmpty(), "no running openshift-apiserver pod found")

		podIP, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("-n", "openshift-apiserver", "pod", podName, "-o", "jsonpath={.status.podIP}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(podIP).NotTo(o.BeEmpty(), "no pod IP found for pod %s", podName)

		g.By("4) Check openshift-apiserver https api metrics endpoint URL for duplicate serial number errors")
		metricsUrl := fmt.Sprintf(`https://%s:8443/metrics`, podIP)
		cmd := fmt.Sprintf(`curl -k --connect-timeout 5 --retry 2 -N -s %s`, metricsUrl)

		metricsOut, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args("-n", "openshift-apiserver", podName, "-c", "openshift-apiserver", "--", "sh", "-c", cmd).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("5) Verify no duplicate serial number errors in metrics")
		o.Expect(metricsOut).ShouldNot(o.ContainSubstring("You are attempting to import a cert with the same issuer/serial as an existing cert, but that is not the same cert"),
			"Found duplicate CA serial number error in openshift-apiserver metrics")
		o.Expect(metricsOut).Should(o.ContainSubstring("Forbidden"),
			"Metrics endpoint should return 'Forbidden' for unauthenticated requests")

		e2e.Logf("No duplicate CA serial number errors found in openshift-apiserver metrics")
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
	kubeClient := oc.AdminKubeClient()
	configClient := oc.AdminConfigClient()

	e2e.Logf("Testing multiple PKI configurations...")

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
		{
			name:       "Mixed-RSA-4096-CA-ECDSA-P384-serving",
			algorithm:  configv1alpha1.KeyAlgorithmECDSA,
			ecdsaCurve: configv1alpha1.ECDSACurveP256,
			signerOverride: &keyOverride{
				algorithm: configv1alpha1.KeyAlgorithmRSA,
				rsaSize:   4096,
			},
			servingOverride: &keyOverride{
				algorithm:  configv1alpha1.KeyAlgorithmECDSA,
				ecdsaCurve: configv1alpha1.ECDSACurveP384,
			},
		},
	}

	for _, tc := range testConfigs {
		e2e.Logf("\n=== Testing configuration: %s ===", tc.name)

		// Apply the PKI configuration
		err := applyPKIConfig(ctx, configClient, tc)
		o.Expect(err).NotTo(o.HaveOccurred(), "error applying PKI config %s", tc.name)

		// Verify the configuration was applied
		pki, err := configClient.ConfigV1alpha1().PKIs().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pki.Spec.CertificateManagement.Mode).To(o.Equal(configv1alpha1.PKICertificateManagementModeCustom))

		e2e.Logf("PKI configuration %s applied successfully", tc.name)

		// Allow the operator to observe the PKI config change before checking
		// Progressing status. PKI config changes do not trigger a full operator
		// rollout (Progressing stays False), so WaitForOperatorToRollout would
		// hang. The actual cert regeneration is validated downstream.
		time.Sleep(10 * time.Second)

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
func testSigningCertRegeneration(ctx context.Context, kubeClient kubernetes.Interface, tc pkiTestConfig) {
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

	regenCtx, regenCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer regenCancel()
	err = waitForSecretRegeneration(regenCtx, kubeClient, "openshift-service-ca", "signing-key", "tls.crt", oldSigningKeyUID)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Verify new certificate matches expected config
	// For signing cert, use signerOverride if present, otherwise use default
	newCert, err := getCertificateFromSecret(ctx, kubeClient, "openshift-service-ca", "signing-key", "tls.crt")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(newCert.IsCA).To(o.BeTrue(), "signing-key certificate should be a CA")

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

	regenCtx2, regenCancel2 := context.WithTimeout(ctx, 15*time.Minute)
	defer regenCancel2()
	err = waitForSecretRegeneration(regenCtx2, kubeClient, "openshift-service-ca-operator", "serving-cert", "tls.crt", oldServingCertUID)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Verify operator's serving cert also uses the new PKI config
	operatorCert, err := getCertificateFromSecret(ctx, kubeClient, "openshift-service-ca-operator", "serving-cert", "tls.crt")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(operatorCert.IsCA).To(o.BeFalse(), "operator serving cert should not be a CA (leaf-first PEM ordering may have changed)")

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
func testServiceCertGeneration(ctx context.Context, kubeClient kubernetes.Interface, tc pkiTestConfig) {
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
	regenCtx, regenCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer regenCancel()
	err = waitForSecretRegeneration(regenCtx, kubeClient, testNS, "test-service-cert", "tls.crt", "")
	o.Expect(err).NotTo(o.HaveOccurred())

	// Verify the service certificate
	serviceCert, err := getCertificateFromSecret(ctx, kubeClient, testNS, "test-service-cert", "tls.crt")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(serviceCert.IsCA).To(o.BeFalse(), "service cert should not be a CA (leaf-first PEM ordering may have changed)")

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
