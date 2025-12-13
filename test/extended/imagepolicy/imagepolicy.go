package imagepolicy

import (
	"context"
	"fmt"
	"net"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	machineconfighelper "github.com/openshift/origin/test/extended/machine_config"
	exutil "github.com/openshift/origin/test/extended/util"
	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	clusterImagePolicyKind                 = "ClusterImagePolicy"
	imagePolicyKind                        = "ImagePolicy"
	testSignedPolicyScope                  = "quay.io/openshifttest/busybox-testsigstoresigned@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f"
	testPKISignedPolicyScope               = "quay.io/openshifttest/busybox-testsigstoresignedpki@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f"
	registriesWorkerPoolMachineConfig      = "99-worker-generated-registries"
	registriesMasterPoolMachineConfig      = "99-master-generated-registries"
	testPodName                            = "signature-validation-test-pod"
	workerPool                             = "worker"
	masterPool                             = "master"
	SignatureValidationFaildReason         = "SignatureValidationFailed"
	invalidPublicKeyClusterImagePolicyName = "invalid-public-key-cluster-image-policy"
	publiKeyRekorClusterImagePolicyName    = "public-key-rekor-cluster-image-policy"
	invalidPublicKeyImagePolicyName        = "invalid-public-key-image-policy"
	publiKeyRekorImagePolicyName           = "public-key-rekor-image-policy"
	invalidPKIClusterImagePolicyName       = "invalid-pki-cluster-image-policy"
	invalidPKIImagePolicyName              = "invalid-pki-image-policy"
	pkiClusterImagePolicyName              = "pki-cluster-image-policy"
	pkiImagePolicyName                     = "pki-image-policy"
	invalidEmailPKIClusterImagePolicyName  = "invalid-email-pki-cluster-image-policy"
)

var _ = g.Describe("[sig-imagepolicy][OCPFeatureGate:SigstoreImageVerification][Serial]", g.Ordered, func() {
	defer g.GinkgoRecover()
	var (
		oc                       = exutil.NewCLIWithoutNamespace("cluster-image-policy")
		tctx                     = context.Background()
		cli                      = exutil.NewCLIWithPodSecurityLevel("verifysigstore-e2e", admissionapi.LevelBaseline)
		clif                     = cli.KubeFramework()
		imgpolicyCli             = exutil.NewCLIWithPodSecurityLevel("verifysigstore-imagepolicy-e2e", admissionapi.LevelBaseline)
		imgpolicyClif            = imgpolicyCli.KubeFramework()
		testClusterImagePolicies = generateClusterImagePolicies()
		testImagePolicies        = generateImagePolicies()
	)

	g.BeforeAll(func() {
		if !exutil.IsTechPreviewNoUpgrade(tctx, oc.AdminConfigClient()) {
			g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
		}
		// skip test on disconnected clusters.
		networkConfig, err := oc.AdminConfigClient().ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Failf("unable to get cluster network config: %v", err)
		}
		usingIPv6 := false
		for _, clusterNetworkEntry := range networkConfig.Status.ClusterNetwork {
			addr, _, err := net.ParseCIDR(clusterNetworkEntry.CIDR)
			if err != nil {
				continue
			}
			if addr.To4() == nil {
				usingIPv6 = true
				break
			}
		}
		if usingIPv6 {
			g.Skip("skipping test on disconnected platform")
		}
	})

	g.It("Should fail clusterimagepolicy signature validation root of trust does not match the identity in the signature", g.Label("Size:M"), func() {
		createClusterImagePolicy(oc, testClusterImagePolicies[invalidPublicKeyClusterImagePolicyName])
		g.DeferCleanup(deleteClusterImagePolicy, oc, invalidPublicKeyClusterImagePolicyName)

		pod, err := launchTestPod(tctx, clif, testPodName, testSignedPolicyScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = waitForTestPodContainerToFailSignatureValidation(tctx, clif, pod)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should fail clusterimagepolicy signature validation when scope in allowedRegistries list does not skip signature verification", g.Label("Size:L"), func() {
		// Ensure allowedRegistries do not skip signature verification by adding testSignedPolicyScope to the list.
		allowedRegistries := []string{"quay.io", "registry.redhat.io", "image-registry.openshift-image-registry.svc:5000", testSignedPolicyScope}
		updateImageConfig(oc, allowedRegistries)
		g.DeferCleanup(cleanupImageConfig, oc)

		createClusterImagePolicy(oc, testClusterImagePolicies[invalidPublicKeyClusterImagePolicyName])
		g.DeferCleanup(deleteClusterImagePolicy, oc, invalidPublicKeyClusterImagePolicyName)

		pod, err := launchTestPod(tctx, clif, testPodName, testSignedPolicyScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = waitForTestPodContainerToFailSignatureValidation(tctx, clif, pod)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should pass clusterimagepolicy signature validation with signed image", g.Label("Size:M"), func() {
		createClusterImagePolicy(oc, testClusterImagePolicies[publiKeyRekorClusterImagePolicyName])
		g.DeferCleanup(deleteClusterImagePolicy, oc, publiKeyRekorClusterImagePolicyName)

		pod, err := launchTestPod(tctx, clif, testPodName, testSignedPolicyScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = e2epod.WaitForPodSuccessInNamespace(tctx, clif.ClientSet, pod.Name, pod.Namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should fail imagepolicy signature validation in different namespaces root of trust does not match the identity in the signature", g.Label("Size:M"), func() {
		createImagePolicy(oc, testImagePolicies[invalidPublicKeyImagePolicyName], imgpolicyClif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, invalidPublicKeyImagePolicyName, imgpolicyClif.Namespace.Name)

		pod, err := launchTestPod(tctx, imgpolicyClif, testPodName, testSignedPolicyScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, imgpolicyClif, testPodName)

		err = waitForTestPodContainerToFailSignatureValidation(tctx, imgpolicyClif, pod)
		o.Expect(err).NotTo(o.HaveOccurred())

	})

	g.It("Should pass imagepolicy signature validation with signed image in namespaces", g.Label("Size:M"), func() {

		createImagePolicy(oc, testImagePolicies[publiKeyRekorImagePolicyName], imgpolicyClif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, publiKeyRekorImagePolicyName, imgpolicyClif.Namespace.Name)

		pod, err := launchTestPod(tctx, imgpolicyClif, testPodName, testSignedPolicyScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, imgpolicyClif, testPodName)

		err = e2epod.WaitForPodSuccessInNamespace(tctx, imgpolicyClif.ClientSet, pod.Name, pod.Namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

var _ = g.Describe("[sig-imagepolicy][OCPFeatureGate:SigstoreImageVerificationPKI][Serial][Skipped:Disconnected]", g.Ordered, func() {
	defer g.GinkgoRecover()
	var (
		oc                       = exutil.NewCLIWithoutNamespace("cluster-image-policy")
		tctx                     = context.Background()
		cli                      = exutil.NewCLIWithPodSecurityLevel("verifysigstore-e2e", admissionapi.LevelBaseline)
		clif                     = cli.KubeFramework()
		imgpolicyCli             = exutil.NewCLIWithPodSecurityLevel("verifysigstore-imagepolicy-e2e", admissionapi.LevelBaseline)
		imgpolicyClif            = imgpolicyCli.KubeFramework()
		testClusterImagePolicies = generateClusterImagePolicies()
		testImagePolicies        = generateImagePolicies()
	)

	g.BeforeAll(func() {
		if !exutil.IsTechPreviewNoUpgrade(tctx, oc.AdminConfigClient()) {
			g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
		}
	})

	g.DescribeTable("clusterimagepolicy signature validation tests",
		func(policyName string, expectPass bool, imageSpec string, verifyFunc func(tctx context.Context, clif *e2e.Framework, expectPass bool, testPodName string, imageSpec string) error) {
			createClusterImagePolicy(oc, testClusterImagePolicies[policyName])
			g.DeferCleanup(deleteClusterImagePolicy, oc, policyName)

			err := verifyFunc(tctx, clif, expectPass, testPodName, imageSpec)
			o.Expect(err).NotTo(o.HaveOccurred())
		},
		g.Entry("fail with PKI root of trust does not match the identity in the signature", g.Label("Size:M"), invalidPKIClusterImagePolicyName, false, testPKISignedPolicyScope, verifyPodSignature),
		g.Entry("fail with PKI email does not match", g.Label("Size:M"), invalidEmailPKIClusterImagePolicyName, false, testPKISignedPolicyScope, verifyPodSignature),
		g.Entry("pass with valid PKI", g.Label("Size:M"), pkiClusterImagePolicyName, true, testPKISignedPolicyScope, verifyPodSignature),
	)

	g.DescribeTable("imagepolicy signature validation tests",
		func(policyName string, expectPass bool, imageSpec string, verifyFunc func(tctx context.Context, clif *e2e.Framework, expectPass bool, testPodName string, imageSpec string) error) {
			createImagePolicy(oc, testImagePolicies[policyName], imgpolicyClif.Namespace.Name)
			g.DeferCleanup(deleteImagePolicy, oc, policyName, imgpolicyClif.Namespace.Name)

			err := verifyFunc(tctx, imgpolicyClif, expectPass, testPodName, imageSpec)
			o.Expect(err).NotTo(o.HaveOccurred())
		},
		g.Entry("fail with PKI root of trust does not match the identity in the signature", g.Label("Size:M"), invalidPKIImagePolicyName, false, testPKISignedPolicyScope, verifyPodSignature),
		g.Entry("pass with valid PKI", g.Label("Size:M"), pkiImagePolicyName, true, testPKISignedPolicyScope, verifyPodSignature),
	)

})

func updateImageConfig(oc *exutil.CLI, allowedRegistries []string) {
	e2e.Logf("Updating image config with allowed registries")
	initialWorkerSpec := getMCPCurrentSpecConfigName(oc, workerPool)
	initialMasterSpec := getMCPCurrentSpecConfigName(oc, masterPool)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(
			context.Background(), "cluster", metav1.GetOptions{},
		)
		if err != nil {
			return err
		}
		imageConfig.Spec.RegistrySources.AllowedRegistries = allowedRegistries
		_, err = oc.AdminConfigClient().ConfigV1().Images().Update(
			context.Background(), imageConfig, metav1.UpdateOptions{},
		)
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "error updating image config")
	waitForMCPConfigSpecChangeAndUpdated(oc, workerPool, initialWorkerSpec)
	waitForMCPConfigSpecChangeAndUpdated(oc, masterPool, initialMasterSpec)
}

func cleanupImageConfig(oc *exutil.CLI) error {
	initialWorkerSpec := getMCPCurrentSpecConfigName(oc, workerPool)
	initialMasterSpec := getMCPCurrentSpecConfigName(oc, masterPool)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		imageConfig, err := oc.AdminConfigClient().ConfigV1().Images().Get(
			context.Background(), "cluster", metav1.GetOptions{},
		)
		if err != nil {
			return err
		}
		imageConfig.Spec.RegistrySources.AllowedRegistries = []string{}
		_, err = oc.AdminConfigClient().ConfigV1().Images().Update(
			context.Background(), imageConfig, metav1.UpdateOptions{},
		)
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "error cleaning up image config")
	waitForMCPConfigSpecChangeAndUpdated(oc, workerPool, initialWorkerSpec)
	waitForMCPConfigSpecChangeAndUpdated(oc, masterPool, initialMasterSpec)
	return nil
}

func launchTestPod(ctx context.Context, f *e2e.Framework, podName, image string) (*kapiv1.Pod, error) {
	g.By(fmt.Sprintf("launching the pod: %s", podName))
	contName := fmt.Sprintf("%s-container", podName)
	pod := &kapiv1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: kapiv1.PodSpec{
			Containers: []kapiv1.Container{
				{
					Name:            contName,
					Image:           image,
					ImagePullPolicy: kapiv1.PullAlways,
					Command:         []string{"/bin/sh", "-c", "exit 0"},
				},
			},
			RestartPolicy: kapiv1.RestartPolicyNever,
		},
	}
	pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
	return pod, err
}

func deleteTestPod(ctx context.Context, f *e2e.Framework, podName string) error {
	return f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(ctx, podName, *metav1.NewDeleteOptions(0))
}

func waitForTestPodContainerToFailSignatureValidation(ctx context.Context, f *e2e.Framework, pod *kapiv1.Pod) error {
	return e2epod.WaitForPodContainerToFail(ctx, f.ClientSet, pod.Namespace, pod.Name, 0, SignatureValidationFaildReason, e2e.PodStartShortTimeout)
}

func createClusterImagePolicy(oc *exutil.CLI, policy configv1.ClusterImagePolicy) {
	e2e.Logf("Creating cluster image policy %s", policy.Name)
	initialWorkerSpec := getMCPCurrentSpecConfigName(oc, workerPool)
	initialMasterSpec := getMCPCurrentSpecConfigName(oc, masterPool)

	_, err := oc.AdminConfigClient().ConfigV1().ClusterImagePolicies().Create(context.TODO(), &policy, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	waitForMCPConfigSpecChangeAndUpdated(oc, workerPool, initialWorkerSpec)
	waitForMCPConfigSpecChangeAndUpdated(oc, masterPool, initialMasterSpec)
}

func deleteClusterImagePolicy(oc *exutil.CLI, policyName string) error {
	initialWorkerSpec := getMCPCurrentSpecConfigName(oc, workerPool)
	initialMasterSpec := getMCPCurrentSpecConfigName(oc, masterPool)

	if err := oc.AdminConfigClient().ConfigV1().ClusterImagePolicies().Delete(context.TODO(), policyName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete cluster image policy %s: %v", policyName, err)
	}
	waitForMCPConfigSpecChangeAndUpdated(oc, workerPool, initialWorkerSpec)
	waitForMCPConfigSpecChangeAndUpdated(oc, masterPool, initialMasterSpec)
	return nil
}

func createImagePolicy(oc *exutil.CLI, policy configv1.ImagePolicy, namespace string) {
	// Capture initial rendered config names for both pools before creating the policy
	initialWorkerSpec := getMCPCurrentSpecConfigName(oc, workerPool)
	initialMasterSpec := getMCPCurrentSpecConfigName(oc, masterPool)

	e2e.Logf("Creating image policy %s in namespace %s", policy.Name, namespace)
	_, err := oc.AdminConfigClient().ConfigV1().ImagePolicies(namespace).Create(context.TODO(), &policy, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Wait until each pool's Spec.Configuration.Name changes from the initial value
	// and the pool reports Updated=true
	waitForMCPConfigSpecChangeAndUpdated(oc, workerPool, initialWorkerSpec)
	waitForMCPConfigSpecChangeAndUpdated(oc, masterPool, initialMasterSpec)
}

func deleteImagePolicy(oc *exutil.CLI, policyName string, namespace string) error {
	initialWorkerSpec := getMCPCurrentSpecConfigName(oc, workerPool)
	initialMasterSpec := getMCPCurrentSpecConfigName(oc, masterPool)

	if err := oc.AdminConfigClient().ConfigV1().ImagePolicies(namespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete image policy %s in namespace %s: %v", policyName, namespace, err)
	}
	waitForMCPConfigSpecChangeAndUpdated(oc, workerPool, initialWorkerSpec)
	waitForMCPConfigSpecChangeAndUpdated(oc, masterPool, initialMasterSpec)
	return nil
}

func generateClusterImagePolicies() map[string]configv1.ClusterImagePolicy {
	testClusterImagePolicies := map[string]configv1.ClusterImagePolicy{
		invalidPublicKeyClusterImagePolicyName: {
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterImagePolicy",
				APIVersion: configv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{Name: invalidPublicKeyClusterImagePolicyName},
			Spec: configv1.ClusterImagePolicySpec{
				Scopes: []configv1.ImageScope{testSignedPolicyScope},
				Policy: configv1.Policy{
					RootOfTrust: configv1.PolicyRootOfTrust{
						PolicyType: configv1.PublicKeyRootOfTrust,
						PublicKey: &configv1.PublicKey{
							KeyData: []byte(`-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEUoFUoYAReKXGy59xe5SQOk2aJ8o+
2/Yz5Y8GcN3zFE6ViIvkGnHhMlAhXaX/bo0M9R62s0/6q++T7uwNFuOg8A==
-----END PUBLIC KEY-----`),
						},
					},
					SignedIdentity: &configv1.PolicyIdentity{
						MatchPolicy: configv1.IdentityMatchPolicyMatchRepoDigestOrExact,
					},
				},
			},
		},
		publiKeyRekorClusterImagePolicyName: {
			TypeMeta: metav1.TypeMeta{
				Kind:       "ClusterImagePolicy",
				APIVersion: configv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{Name: publiKeyRekorClusterImagePolicyName},
			Spec: configv1.ClusterImagePolicySpec{
				Scopes: []configv1.ImageScope{testSignedPolicyScope},
				Policy: configv1.Policy{
					RootOfTrust: configv1.PolicyRootOfTrust{
						PolicyType: configv1.PublicKeyRootOfTrust,
						PublicKey: &configv1.PublicKey{
							KeyData: []byte(`-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEKvZH0CXTk8XQkETuxkzkl3Bi4ms5
60l1/qUU0fRATNSCVORCog5PDFo5z0ZLeblWgwbn4c8xpvuo9jQFwpeOsg==
-----END PUBLIC KEY-----`),
						},
					},
					SignedIdentity: &configv1.PolicyIdentity{
						MatchPolicy: configv1.IdentityMatchPolicyMatchRepository,
					},
				},
			},
		},
		invalidPKIClusterImagePolicyName: {
			TypeMeta: metav1.TypeMeta{
				Kind:       clusterImagePolicyKind,
				APIVersion: configv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{Name: invalidPKIClusterImagePolicyName},
			Spec: configv1.ClusterImagePolicySpec{
				Scopes: []configv1.ImageScope{testPKISignedPolicyScope},
				Policy: configv1.Policy{
					RootOfTrust: configv1.PolicyRootOfTrust{
						PolicyType: configv1.PKIRootOfTrust,
						PKI: &configv1.PKI{
							CertificateAuthorityRootsData: []byte(`-----BEGIN CERTIFICATE-----
MIICYDCCAgagAwIBAgIUTq5IQKTGqI9XDqGzdGzm8mI43qkwCgYIKoZIzj0EAwIw
fDELMAkGA1UEBhMCLS0xDjAMBgNVBAgTBVNUQVRFMREwDwYDVQQHEwhMT0NBTElU
WTEVMBMGA1UEChMMT1JHQU5JU0FUSU9OMQ4wDAYDVQQLEwVMT0NBTDEjMCEGA1UE
AxMaUm9vdCBDZXJ0aWZpY2F0ZSBBdXRob3JpdHkwHhcNMjQwNjA2MTQxODAwWhcN
MzQwNjA0MTQxODAwWjB8MQswCQYDVQQGEwItLTEOMAwGA1UECBMFU1RBVEUxETAP
BgNVBAcTCExPQ0FMSVRZMRUwEwYDVQQKEwxPUkdBTklTQVRJT04xDjAMBgNVBAsT
BUxPQ0FMMSMwIQYDVQQDExpSb290IENlcnRpZmljYXRlIEF1dGhvcml0eTBZMBMG
ByqGSM49AgEGCCqGSM49AwEHA0IABDYxY1BnzNsriTp9PZ0TSumXOg36Xr4fO6xa
RHp7chgZ9KUhA+s2YoafOWobSiq3ZhfU5vjT2MVIeJjOZjw9EUWjZjBkMA4GA1Ud
DwEB/wQEAwIBBjASBgNVHRMBAf8ECDAGAQH/AgECMB0GA1UdDgQWBBQQOPL7R8z2
dG1h6uJ6bWX/xxl6mjAfBgNVHSMEGDAWgBQQOPL7R8z2dG1h6uJ6bWX/xxl6mjAK
BggqhkjOPQQDAgNIADBFAiAf7kYcHVNe1kj6R8pdVlAckVZZTu6khmBlJoe32FEu
TAIhALlR4yZRRYv2iaVPdgaptAI0LoDAtEUiO8Rb9FWJzpAN
-----END CERTIFICATE-----`),
							PKICertificateSubject: configv1.PKICertificateSubject{
								Email: "team-a@linuxera.org",
							},
						},
					},
					SignedIdentity: &configv1.PolicyIdentity{
						MatchPolicy: configv1.IdentityMatchPolicyMatchRepository,
					},
				},
			},
		},
		pkiClusterImagePolicyName: {
			TypeMeta: metav1.TypeMeta{
				Kind:       clusterImagePolicyKind,
				APIVersion: configv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{Name: pkiClusterImagePolicyName},
			Spec: configv1.ClusterImagePolicySpec{
				Scopes: []configv1.ImageScope{testPKISignedPolicyScope},
				Policy: configv1.Policy{
					RootOfTrust: configv1.PolicyRootOfTrust{
						PolicyType: configv1.PKIRootOfTrust,
						PKI: &configv1.PKI{
							CertificateAuthorityRootsData: []byte(`-----BEGIN CERTIFICATE-----
MIIFvzCCA6egAwIBAgIUZnH3ITyYQMAp6lvNYc0fjRzzuBcwDQYJKoZIhvcNAQEL
BQAwbjELMAkGA1UEBhMCRVMxETAPBgNVBAcMCFZhbGVuY2lhMQswCQYDVQQKDAJJ
VDERMA8GA1UECwwIU2VjdXJpdHkxLDAqBgNVBAMMI0xpbnV4ZXJhIFJvb3QgQ2Vy
dGlmaWNhdGUgQXV0aG9yaXR5MCAXDTI0MDkzMDE2MjM1N1oYDzIwNTIwMjE1MTYy
MzU3WjBuMQswCQYDVQQGEwJFUzERMA8GA1UEBwwIVmFsZW5jaWExCzAJBgNVBAoM
AklUMREwDwYDVQQLDAhTZWN1cml0eTEsMCoGA1UEAwwjTGludXhlcmEgUm9vdCBD
ZXJ0aWZpY2F0ZSBBdXRob3JpdHkwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIK
AoICAQCy8vGuh6+27xqtsANJUMIeGaX/rjx5hIgh/eOcxZc2/azTB/zHnwjZX7qn
Co3zaYZaS3ibOouS1yPv2G3NeRPwfGHn2kcR3QM7h4BdYxZ3SR/VioaWpVymLCm2
/V2gQWMWKrtdYfOXBviqhhD9OIxrLSOqjac8T/icQcfN+dKktKyGlY7vJLKO9w2x
IdpOTa2IDuYp5DNQV6vy9sDFglP/iafvcDkLGUhrsop8LeNcejpmpFBPRwJKXgan
5spry6GgCpNNJuB/Hqgth0fGPjMEY8bPuVOCehnRxe094U01sGrobkkbnM+SxumA
oLwk1//jC1K3HaKjkIOMMHxEzqYx0Q4RalvPWhd6o/KP5Cs+rd5+EwSeFuvbaIrF
sEPZBPpH0UDLR0yiQNk2j4LVbV1xdP7tX8KtUvF8+E3Gm5SwnCodNbfnAUxNF4RK
4lDqGibUUI5B5SniJ5YMVeTJSc1Jo9gTaKa9lRniMitY9FjzjQjDF4yGnhNPmmKG
zIvVOXIhQpcw3UhEMmDz6p1wr3wMDtjufoaxaTjoAuxUzSwwFqxzzcJenQiHoFeQ
B6cJ5RayizadlkqBnHAkrzAB0aM9W8zh5AhIcnO6gfGBaOFom+I5Huy3TyZ9FjTn
vlxVM5txPV5VsBPMK96hF6mnWeKNg/22qY0X+wo8T33G4LvWIwIDAQABo1MwUTAd
BgNVHQ4EFgQUD+bFpMAOhNSptdQo+NZle+Yd1L4wHwYDVR0jBBgwFoAUD+bFpMAO
hNSptdQo+NZle+Yd1L4wDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOC
AgEAmE21e2H51volFI0CboDakb5T9VLkDzLgmxH2iZPBJrnQBFaPTEaQnM93pDq+
czfc7+WJL+6TUyUYFOg2rueK/KWC3AQYUrsb+i3BDNZVv74f3wLidmqELcyjHO8m
7yoGIgeG8ksMYPCzPfuuFHYNDiv11brmbdhdGGbvQMbayLYvhB543J5sTiUsr3iv
ShKvmr/krAbdj6ZK2m6us+pFktjjbirHVqj5tE+RvEC9oHSngyCRCKJEuEDt+gUK
gmSFh1+AFJdjWqYqnX7kPu6N4x4KoH72OUkd7NHpzkG57UM0iVQ8jCAclkZxrpng
HCD+dY0JnIlF+LJ7qGgmrNQQvTZ11hWyV7fRHcCPwuqT0kJC/yjWWXEafsMWTPl7
2zrQg5YW0zbcWfRzo1ucx0tf47unRjVqjaXjyyzkgkHrqZH939SrAy9e2SFZUqdy
qIXwGmZktzL8DU+8ZH47R+CIwcv59l4Wy889fUrjk4Kgg45IhqnP5NMg2Z8aytUH
0Zwo0iJxuCe0tQTdSMvYC0PoWsEyR4KULEU83GfCbGZQG8hOFAPHXV0CpM025+9Y
L8ITFP+Nw9Meiw4etw59CTAPCc7l4Zvwr1K2ZTBmVGxrqdasiqpI0utG69aItsPi
+9V8SSde7D5iMV/3z9LDxA/oLoqNGFcD0TSR5+obeqJzl40=
-----END CERTIFICATE-----`),
							PKICertificateSubject: configv1.PKICertificateSubject{
								Email: "team-a@linuxera.org",
							},
						},
					},
					SignedIdentity: &configv1.PolicyIdentity{
						MatchPolicy: configv1.IdentityMatchPolicyMatchRepository,
					},
				},
			},
		},
		invalidEmailPKIClusterImagePolicyName: {
			TypeMeta: metav1.TypeMeta{
				Kind:       clusterImagePolicyKind,
				APIVersion: configv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{Name: invalidEmailPKIClusterImagePolicyName},
			Spec: configv1.ClusterImagePolicySpec{
				Scopes: []configv1.ImageScope{testPKISignedPolicyScope},
				Policy: configv1.Policy{
					RootOfTrust: configv1.PolicyRootOfTrust{
						PolicyType: configv1.PKIRootOfTrust,
						PKI: &configv1.PKI{
							CertificateAuthorityRootsData: []byte(`-----BEGIN CERTIFICATE-----
MIIFvzCCA6egAwIBAgIUZnH3ITyYQMAp6lvNYc0fjRzzuBcwDQYJKoZIhvcNAQEL
BQAwbjELMAkGA1UEBhMCRVMxETAPBgNVBAcMCFZhbGVuY2lhMQswCQYDVQQKDAJJ
VDERMA8GA1UECwwIU2VjdXJpdHkxLDAqBgNVBAMMI0xpbnV4ZXJhIFJvb3QgQ2Vy
dGlmaWNhdGUgQXV0aG9yaXR5MCAXDTI0MDkzMDE2MjM1N1oYDzIwNTIwMjE1MTYy
MzU3WjBuMQswCQYDVQQGEwJFUzERMA8GA1UEBwwIVmFsZW5jaWExCzAJBgNVBAoM
AklUMREwDwYDVQQLDAhTZWN1cml0eTEsMCoGA1UEAwwjTGludXhlcmEgUm9vdCBD
ZXJ0aWZpY2F0ZSBBdXRob3JpdHkwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIK
AoICAQCy8vGuh6+27xqtsANJUMIeGaX/rjx5hIgh/eOcxZc2/azTB/zHnwjZX7qn
Co3zaYZaS3ibOouS1yPv2G3NeRPwfGHn2kcR3QM7h4BdYxZ3SR/VioaWpVymLCm2
/V2gQWMWKrtdYfOXBviqhhD9OIxrLSOqjac8T/icQcfN+dKktKyGlY7vJLKO9w2x
IdpOTa2IDuYp5DNQV6vy9sDFglP/iafvcDkLGUhrsop8LeNcejpmpFBPRwJKXgan
5spry6GgCpNNJuB/Hqgth0fGPjMEY8bPuVOCehnRxe094U01sGrobkkbnM+SxumA
oLwk1//jC1K3HaKjkIOMMHxEzqYx0Q4RalvPWhd6o/KP5Cs+rd5+EwSeFuvbaIrF
sEPZBPpH0UDLR0yiQNk2j4LVbV1xdP7tX8KtUvF8+E3Gm5SwnCodNbfnAUxNF4RK
4lDqGibUUI5B5SniJ5YMVeTJSc1Jo9gTaKa9lRniMitY9FjzjQjDF4yGnhNPmmKG
zIvVOXIhQpcw3UhEMmDz6p1wr3wMDtjufoaxaTjoAuxUzSwwFqxzzcJenQiHoFeQ
B6cJ5RayizadlkqBnHAkrzAB0aM9W8zh5AhIcnO6gfGBaOFom+I5Huy3TyZ9FjTn
vlxVM5txPV5VsBPMK96hF6mnWeKNg/22qY0X+wo8T33G4LvWIwIDAQABo1MwUTAd
BgNVHQ4EFgQUD+bFpMAOhNSptdQo+NZle+Yd1L4wHwYDVR0jBBgwFoAUD+bFpMAO
hNSptdQo+NZle+Yd1L4wDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOC
AgEAmE21e2H51volFI0CboDakb5T9VLkDzLgmxH2iZPBJrnQBFaPTEaQnM93pDq+
czfc7+WJL+6TUyUYFOg2rueK/KWC3AQYUrsb+i3BDNZVv74f3wLidmqELcyjHO8m
7yoGIgeG8ksMYPCzPfuuFHYNDiv11brmbdhdGGbvQMbayLYvhB543J5sTiUsr3iv
ShKvmr/krAbdj6ZK2m6us+pFktjjbirHVqj5tE+RvEC9oHSngyCRCKJEuEDt+gUK
gmSFh1+AFJdjWqYqnX7kPu6N4x4KoH72OUkd7NHpzkG57UM0iVQ8jCAclkZxrpng
HCD+dY0JnIlF+LJ7qGgmrNQQvTZ11hWyV7fRHcCPwuqT0kJC/yjWWXEafsMWTPl7
2zrQg5YW0zbcWfRzo1ucx0tf47unRjVqjaXjyyzkgkHrqZH939SrAy9e2SFZUqdy
qIXwGmZktzL8DU+8ZH47R+CIwcv59l4Wy889fUrjk4Kgg45IhqnP5NMg2Z8aytUH
0Zwo0iJxuCe0tQTdSMvYC0PoWsEyR4KULEU83GfCbGZQG8hOFAPHXV0CpM025+9Y
L8ITFP+Nw9Meiw4etw59CTAPCc7l4Zvwr1K2ZTBmVGxrqdasiqpI0utG69aItsPi
+9V8SSde7D5iMV/3z9LDxA/oLoqNGFcD0TSR5+obeqJzl40=
-----END CERTIFICATE-----`),
							PKICertificateSubject: configv1.PKICertificateSubject{
								Email: "testuser@example.com",
							},
						},
					},
					SignedIdentity: &configv1.PolicyIdentity{
						MatchPolicy: configv1.IdentityMatchPolicyMatchRepository,
					},
				},
			},
		},
	}
	return testClusterImagePolicies
}

func generateImagePolicies() map[string]configv1.ImagePolicy {
	testImagePolicies := map[string]configv1.ImagePolicy{
		invalidPublicKeyImagePolicyName: {
			TypeMeta: metav1.TypeMeta{
				Kind:       "ImagePolicy",
				APIVersion: configv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{Name: invalidPublicKeyImagePolicyName},
			Spec: configv1.ImagePolicySpec{
				Scopes: []configv1.ImageScope{testSignedPolicyScope},
				Policy: configv1.Policy{
					RootOfTrust: configv1.PolicyRootOfTrust{
						PolicyType: configv1.PublicKeyRootOfTrust,
						PublicKey: &configv1.PublicKey{
							KeyData: []byte(`-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEUoFUoYAReKXGy59xe5SQOk2aJ8o+
2/Yz5Y8GcN3zFE6ViIvkGnHhMlAhXaX/bo0M9R62s0/6q++T7uwNFuOg8A==
-----END PUBLIC KEY-----`),
						},
					},
					SignedIdentity: &configv1.PolicyIdentity{
						MatchPolicy: configv1.IdentityMatchPolicyMatchRepoDigestOrExact,
					},
				},
			},
		},
		publiKeyRekorImagePolicyName: {
			TypeMeta: metav1.TypeMeta{
				Kind:       "ImagePolicy",
				APIVersion: configv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{Name: publiKeyRekorImagePolicyName},
			Spec: configv1.ImagePolicySpec{
				Scopes: []configv1.ImageScope{testSignedPolicyScope},
				Policy: configv1.Policy{
					RootOfTrust: configv1.PolicyRootOfTrust{
						PolicyType: configv1.PublicKeyRootOfTrust,
						PublicKey: &configv1.PublicKey{
							KeyData: []byte(`-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEKvZH0CXTk8XQkETuxkzkl3Bi4ms5
60l1/qUU0fRATNSCVORCog5PDFo5z0ZLeblWgwbn4c8xpvuo9jQFwpeOsg==
-----END PUBLIC KEY-----`),
						},
					},
					SignedIdentity: &configv1.PolicyIdentity{
						MatchPolicy: configv1.IdentityMatchPolicyMatchRepository,
					},
				},
			},
		},
		invalidPKIImagePolicyName: {
			TypeMeta: metav1.TypeMeta{
				Kind:       imagePolicyKind,
				APIVersion: configv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{Name: invalidPKIImagePolicyName},
			Spec: configv1.ImagePolicySpec{
				Scopes: []configv1.ImageScope{testPKISignedPolicyScope},
				Policy: configv1.Policy{
					RootOfTrust: configv1.PolicyRootOfTrust{
						PolicyType: configv1.PKIRootOfTrust,
						PKI: &configv1.PKI{
							CertificateAuthorityRootsData: []byte(`-----BEGIN CERTIFICATE-----
MIICYDCCAgagAwIBAgIUTq5IQKTGqI9XDqGzdGzm8mI43qkwCgYIKoZIzj0EAwIw
fDELMAkGA1UEBhMCLS0xDjAMBgNVBAgTBVNUQVRFMREwDwYDVQQHEwhMT0NBTElU
WTEVMBMGA1UEChMMT1JHQU5JU0FUSU9OMQ4wDAYDVQQLEwVMT0NBTDEjMCEGA1UE
AxMaUm9vdCBDZXJ0aWZpY2F0ZSBBdXRob3JpdHkwHhcNMjQwNjA2MTQxODAwWhcN
MzQwNjA0MTQxODAwWjB8MQswCQYDVQQGEwItLTEOMAwGA1UECBMFU1RBVEUxETAP
BgNVBAcTCExPQ0FMSVRZMRUwEwYDVQQKEwxPUkdBTklTQVRJT04xDjAMBgNVBAsT
BUxPQ0FMMSMwIQYDVQQDExpSb290IENlcnRpZmljYXRlIEF1dGhvcml0eTBZMBMG
ByqGSM49AgEGCCqGSM49AwEHA0IABDYxY1BnzNsriTp9PZ0TSumXOg36Xr4fO6xa
RHp7chgZ9KUhA+s2YoafOWobSiq3ZhfU5vjT2MVIeJjOZjw9EUWjZjBkMA4GA1Ud
DwEB/wQEAwIBBjASBgNVHRMBAf8ECDAGAQH/AgECMB0GA1UdDgQWBBQQOPL7R8z2
dG1h6uJ6bWX/xxl6mjAfBgNVHSMEGDAWgBQQOPL7R8z2dG1h6uJ6bWX/xxl6mjAK
BggqhkjOPQQDAgNIADBFAiAf7kYcHVNe1kj6R8pdVlAckVZZTu6khmBlJoe32FEu
TAIhALlR4yZRRYv2iaVPdgaptAI0LoDAtEUiO8Rb9FWJzpAN
-----END CERTIFICATE-----`),
							PKICertificateSubject: configv1.PKICertificateSubject{
								Email: "team-a@linuxera.org",
							},
						},
					},
					SignedIdentity: &configv1.PolicyIdentity{
						MatchPolicy: configv1.IdentityMatchPolicyMatchRepository,
					},
				},
			},
		},
		pkiImagePolicyName: {
			TypeMeta: metav1.TypeMeta{
				Kind:       imagePolicyKind,
				APIVersion: configv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{Name: pkiImagePolicyName},
			Spec: configv1.ImagePolicySpec{
				Scopes: []configv1.ImageScope{testPKISignedPolicyScope},
				Policy: configv1.Policy{
					RootOfTrust: configv1.PolicyRootOfTrust{
						PolicyType: configv1.PKIRootOfTrust,
						PKI: &configv1.PKI{
							CertificateAuthorityRootsData: []byte(`-----BEGIN CERTIFICATE-----
MIIFvzCCA6egAwIBAgIUZnH3ITyYQMAp6lvNYc0fjRzzuBcwDQYJKoZIhvcNAQEL
BQAwbjELMAkGA1UEBhMCRVMxETAPBgNVBAcMCFZhbGVuY2lhMQswCQYDVQQKDAJJ
VDERMA8GA1UECwwIU2VjdXJpdHkxLDAqBgNVBAMMI0xpbnV4ZXJhIFJvb3QgQ2Vy
dGlmaWNhdGUgQXV0aG9yaXR5MCAXDTI0MDkzMDE2MjM1N1oYDzIwNTIwMjE1MTYy
MzU3WjBuMQswCQYDVQQGEwJFUzERMA8GA1UEBwwIVmFsZW5jaWExCzAJBgNVBAoM
AklUMREwDwYDVQQLDAhTZWN1cml0eTEsMCoGA1UEAwwjTGludXhlcmEgUm9vdCBD
ZXJ0aWZpY2F0ZSBBdXRob3JpdHkwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIK
AoICAQCy8vGuh6+27xqtsANJUMIeGaX/rjx5hIgh/eOcxZc2/azTB/zHnwjZX7qn
Co3zaYZaS3ibOouS1yPv2G3NeRPwfGHn2kcR3QM7h4BdYxZ3SR/VioaWpVymLCm2
/V2gQWMWKrtdYfOXBviqhhD9OIxrLSOqjac8T/icQcfN+dKktKyGlY7vJLKO9w2x
IdpOTa2IDuYp5DNQV6vy9sDFglP/iafvcDkLGUhrsop8LeNcejpmpFBPRwJKXgan
5spry6GgCpNNJuB/Hqgth0fGPjMEY8bPuVOCehnRxe094U01sGrobkkbnM+SxumA
oLwk1//jC1K3HaKjkIOMMHxEzqYx0Q4RalvPWhd6o/KP5Cs+rd5+EwSeFuvbaIrF
sEPZBPpH0UDLR0yiQNk2j4LVbV1xdP7tX8KtUvF8+E3Gm5SwnCodNbfnAUxNF4RK
4lDqGibUUI5B5SniJ5YMVeTJSc1Jo9gTaKa9lRniMitY9FjzjQjDF4yGnhNPmmKG
zIvVOXIhQpcw3UhEMmDz6p1wr3wMDtjufoaxaTjoAuxUzSwwFqxzzcJenQiHoFeQ
B6cJ5RayizadlkqBnHAkrzAB0aM9W8zh5AhIcnO6gfGBaOFom+I5Huy3TyZ9FjTn
vlxVM5txPV5VsBPMK96hF6mnWeKNg/22qY0X+wo8T33G4LvWIwIDAQABo1MwUTAd
BgNVHQ4EFgQUD+bFpMAOhNSptdQo+NZle+Yd1L4wHwYDVR0jBBgwFoAUD+bFpMAO
hNSptdQo+NZle+Yd1L4wDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOC
AgEAmE21e2H51volFI0CboDakb5T9VLkDzLgmxH2iZPBJrnQBFaPTEaQnM93pDq+
czfc7+WJL+6TUyUYFOg2rueK/KWC3AQYUrsb+i3BDNZVv74f3wLidmqELcyjHO8m
7yoGIgeG8ksMYPCzPfuuFHYNDiv11brmbdhdGGbvQMbayLYvhB543J5sTiUsr3iv
ShKvmr/krAbdj6ZK2m6us+pFktjjbirHVqj5tE+RvEC9oHSngyCRCKJEuEDt+gUK
gmSFh1+AFJdjWqYqnX7kPu6N4x4KoH72OUkd7NHpzkG57UM0iVQ8jCAclkZxrpng
HCD+dY0JnIlF+LJ7qGgmrNQQvTZ11hWyV7fRHcCPwuqT0kJC/yjWWXEafsMWTPl7
2zrQg5YW0zbcWfRzo1ucx0tf47unRjVqjaXjyyzkgkHrqZH939SrAy9e2SFZUqdy
qIXwGmZktzL8DU+8ZH47R+CIwcv59l4Wy889fUrjk4Kgg45IhqnP5NMg2Z8aytUH
0Zwo0iJxuCe0tQTdSMvYC0PoWsEyR4KULEU83GfCbGZQG8hOFAPHXV0CpM025+9Y
L8ITFP+Nw9Meiw4etw59CTAPCc7l4Zvwr1K2ZTBmVGxrqdasiqpI0utG69aItsPi
+9V8SSde7D5iMV/3z9LDxA/oLoqNGFcD0TSR5+obeqJzl40=
-----END CERTIFICATE-----`),
							PKICertificateSubject: configv1.PKICertificateSubject{
								Email: "team-a@linuxera.org",
							},
						},
					},
					SignedIdentity: &configv1.PolicyIdentity{
						MatchPolicy: configv1.IdentityMatchPolicyMatchRepository,
					},
				},
			},
		},
	}
	return testImagePolicies
}

// getMCPCurrentSpecConfigName returns the current Spec.Configuration.Name for the given MCP
func getMCPCurrentSpecConfigName(oc *exutil.CLI, pool string) string {
	clientSet, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	mcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), pool, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return mcp.Spec.Configuration.Name
}

// waitForMCPConfigSpecChangeAndUpdated waits until Spec.Configuration.Name changes from the provided initial value
// and the MCP reports Updated=true
func waitForMCPConfigSpecChangeAndUpdated(oc *exutil.CLI, pool string, initialSpecName string) {
	e2e.Logf("Waiting for pool %s to complete", pool)
	clientSet, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Eventually(func() bool {
		mcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), pool, metav1.GetOptions{})
		if err != nil {
			return false
		}
		if mcp.Status.Configuration.Name == initialSpecName {
			return false
		}
		return machineconfighelper.IsMachineConfigPoolConditionTrue(mcp.Status.Conditions, mcfgv1.MachineConfigPoolUpdated)
	}, 20*time.Minute, 10*time.Second).Should(o.BeTrue())
}

func isDisconnectedCluster(oc *exutil.CLI) bool {
	networkConfig, err := oc.AdminConfigClient().ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		e2e.Failf("unable to get cluster network config: %v", err)
	}
	usingIPv6 := false
	for _, clusterNetworkEntry := range networkConfig.Status.ClusterNetwork {
		addr, _, err := net.ParseCIDR(clusterNetworkEntry.CIDR)
		if err != nil {
			continue
		}
		if addr.To4() == nil {
			usingIPv6 = true
			break
		}
	}
	return usingIPv6
}

func verifyPodSignature(tctx context.Context, clif *e2e.Framework, expectPass bool, testPodName string, imageSpec string) error {
	pod, err := launchTestPod(tctx, clif, testPodName, imageSpec)
	if err != nil {
		return err
	}
	g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

	if expectPass {
		return e2epod.WaitForPodSuccessInNamespace(tctx, clif.ClientSet, pod.Name, pod.Namespace)
	} else {
		return waitForTestPodContainerToFailSignatureValidation(tctx, clif, pod)
	}
}
