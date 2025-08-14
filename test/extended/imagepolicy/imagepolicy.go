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
	testSignedPolicyScope                  = "quay.io/openshifttest/busybox-testsigstoresigned@sha256:c5439d7db88ab5423999530349d327b04279ad3161d7596d2126dfb5b02bfd1f"
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

	g.It("Should fail clusterimagepolicy signature validation root of trust does not match the identity in the signature", func() {
		createClusterImagePolicy(oc, testClusterImagePolicies[invalidPublicKeyClusterImagePolicyName])
		g.DeferCleanup(deleteClusterImagePolicy, oc, invalidPublicKeyClusterImagePolicyName)

		pod, err := launchTestPod(tctx, clif, testPodName, testSignedPolicyScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = waitForTestPodContainerToFailSignatureValidation(tctx, clif, pod)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should fail clusterimagepolicy signature validation when scope in allowedRegistries list does not skip signature verification", func() {
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

	g.It("Should pass clusterimagepolicy signature validation with signed image", func() {
		createClusterImagePolicy(oc, testClusterImagePolicies[publiKeyRekorClusterImagePolicyName])
		g.DeferCleanup(deleteClusterImagePolicy, oc, publiKeyRekorClusterImagePolicyName)

		pod, err := launchTestPod(tctx, clif, testPodName, testSignedPolicyScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = e2epod.WaitForPodSuccessInNamespace(tctx, clif.ClientSet, pod.Name, pod.Namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should fail imagepolicy signature validation in different namespaces root of trust does not match the identity in the signature", func() {
		createImagePolicy(oc, testImagePolicies[invalidPublicKeyImagePolicyName], imgpolicyClif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, invalidPublicKeyImagePolicyName, imgpolicyClif.Namespace.Name)

		pod, err := launchTestPod(tctx, imgpolicyClif, testPodName, testSignedPolicyScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, imgpolicyClif, testPodName)

		err = waitForTestPodContainerToFailSignatureValidation(tctx, imgpolicyClif, pod)
		o.Expect(err).NotTo(o.HaveOccurred())

	})

	g.It("Should pass imagepolicy signature validation with signed image in namespaces", func() {

		createImagePolicy(oc, testImagePolicies[publiKeyRekorImagePolicyName], imgpolicyClif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, publiKeyRekorImagePolicyName, imgpolicyClif.Namespace.Name)

		pod, err := launchTestPod(tctx, imgpolicyClif, testPodName, testSignedPolicyScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, imgpolicyClif, testPodName)

		err = e2epod.WaitForPodSuccessInNamespace(tctx, imgpolicyClif.ClientSet, pod.Name, pod.Namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
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
