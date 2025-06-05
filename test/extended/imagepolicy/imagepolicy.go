package imagepolicy

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	machineconfighelper "github.com/openshift/origin/test/extended/machine_config"
	exutil "github.com/openshift/origin/test/extended/util"
	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	testReleaseImageScope                  = "quay.io/openshift-release-dev/ocp-release@sha256:fbad931c725b2e5b937b295b58345334322bdabb0b67da1c800a53686d7397da"
	testReferenceImageScope                = "quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:4db234f37ae6712e2f7ed8d13f7fb49971c173d0e4f74613d0121672fa2e01f5"
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

		outStr, err := oc.Run("adm", "release", "info", testReleaseImageScope).Args("-o=go-template", "--template={{.digest}}").Output()
		if err != nil || outStr == "" {
			o.Expect(err).ToNot(o.HaveOccurred())
			e2eskipper.Skipf("can't validate %s release image for testing, consider updating the test", testReleaseImageScope)
		}
	})

	g.It("Should fail clusterimagepolicy signature validation root of trust does not match the identity in the signature", func() {
		createClusterImagePolicy(oc, testClusterImagePolicies[invalidPublicKeyClusterImagePolicyName])
		g.DeferCleanup(deleteClusterImagePolicy, oc, invalidPublicKeyClusterImagePolicyName)

		waitForPoolComplete(oc)

		pod, err := launchTestPod(tctx, clif, testPodName, testReleaseImageScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = waitForTestPodContainerToFailSignatureValidation(tctx, clif, pod)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should fail clusterimagepolicy signature validation when scope in allowedRegistries list does not skip signature verification", func() {
		// Ensure allowedRegistries do not skip signature verification by adding testReleaseImageScope to the list
		allowedRegistries := []string{"quay.io", "registry.redhat.io", "image-registry.openshift-image-registry.svc:5000", testReleaseImageScope}
		updateImageConfig(oc, allowedRegistries)
		g.DeferCleanup(cleanupImageConfig, oc)

		createClusterImagePolicy(oc, testClusterImagePolicies[invalidPublicKeyClusterImagePolicyName])
		g.DeferCleanup(deleteClusterImagePolicy, oc, invalidPublicKeyClusterImagePolicyName)

		waitForPoolComplete(oc)

		pod, err := launchTestPod(tctx, clif, testPodName, testReleaseImageScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = waitForTestPodContainerToFailSignatureValidation(tctx, clif, pod)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should pass clusterimagepolicy signature validation with signed image", func() {
		createClusterImagePolicy(oc, testClusterImagePolicies[publiKeyRekorClusterImagePolicyName])
		g.DeferCleanup(deleteClusterImagePolicy, oc, publiKeyRekorClusterImagePolicyName)

		waitForPoolComplete(oc)

		pod, err := launchTestPod(tctx, clif, testPodName, testReleaseImageScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = e2epod.WaitForPodSuccessInNamespace(tctx, clif.ClientSet, pod.Name, pod.Namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should fail imagepolicy signature validation in different namespaces root of trust does not match the identity in the signature", func() {
		createImagePolicy(oc, testImagePolicies[invalidPublicKeyImagePolicyName], imgpolicyClif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, invalidPublicKeyImagePolicyName, imgpolicyClif.Namespace.Name)

		createImagePolicy(oc, testImagePolicies[invalidPublicKeyImagePolicyName], clif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, invalidPublicKeyImagePolicyName, clif.Namespace.Name)

		waitForPoolComplete(oc)

		pod, err := launchTestPod(tctx, imgpolicyClif, testPodName, testReferenceImageScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, imgpolicyClif, testPodName)

		err = waitForTestPodContainerToFailSignatureValidation(tctx, imgpolicyClif, pod)
		o.Expect(err).NotTo(o.HaveOccurred())

		pod, err = launchTestPod(tctx, clif, testPodName, testReferenceImageScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = waitForTestPodContainerToFailSignatureValidation(tctx, clif, pod)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should pass imagepolicy signature validation with signed image in namespaces", func() {
		createImagePolicy(oc, testImagePolicies[publiKeyRekorImagePolicyName], clif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, publiKeyRekorImagePolicyName, clif.Namespace.Name)

		createImagePolicy(oc, testImagePolicies[publiKeyRekorImagePolicyName], imgpolicyClif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, publiKeyRekorImagePolicyName, imgpolicyClif.Namespace.Name)

		waitForPoolComplete(oc)

		pod, err := launchTestPod(tctx, clif, testPodName, testReferenceImageScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = e2epod.WaitForPodSuccessInNamespace(tctx, clif.ClientSet, pod.Name, pod.Namespace)
		o.Expect(err).NotTo(o.HaveOccurred())

		pod, err = launchTestPod(tctx, imgpolicyClif, testPodName, testReferenceImageScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, imgpolicyClif, testPodName)

		err = e2epod.WaitForPodSuccessInNamespace(tctx, imgpolicyClif.ClientSet, pod.Name, pod.Namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

func waitForPoolComplete(oc *exutil.CLI) {
	time.Sleep(10 * time.Second)
	machineconfighelper.WaitForConfigAndPoolComplete(oc, workerPool, registriesWorkerPoolMachineConfig)
	machineconfighelper.WaitForConfigAndPoolComplete(oc, masterPool, registriesMasterPoolMachineConfig)
}

func updateImageConfig(oc *exutil.CLI, allowedRegistries []string) {
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
	time.Sleep(10 * time.Second)
	machineconfighelper.WaitForConfigAndPoolComplete(oc, workerPool, registriesWorkerPoolMachineConfig)
	machineconfighelper.WaitForConfigAndPoolComplete(oc, masterPool, registriesMasterPoolMachineConfig)
}

func cleanupImageConfig(oc *exutil.CLI) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
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
		waitForPoolComplete(oc)
		return err
	})
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

func createClusterImagePolicy(oc *exutil.CLI, policy configv1alpha1.ClusterImagePolicy) {
	_, err := oc.AdminConfigClient().ConfigV1alpha1().ClusterImagePolicies().Create(context.TODO(), &policy, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func deleteClusterImagePolicy(oc *exutil.CLI, policyName string) error {
	if err := oc.AdminConfigClient().ConfigV1alpha1().ClusterImagePolicies().Delete(context.TODO(), policyName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete cluster image policy %s: %v", policyName, err)
	}
	waitForPoolComplete(oc)
	return nil
}

func createImagePolicy(oc *exutil.CLI, policy configv1alpha1.ImagePolicy, namespace string) {
	_, err := oc.AdminConfigClient().ConfigV1alpha1().ImagePolicies(namespace).Create(context.TODO(), &policy, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func deleteImagePolicy(oc *exutil.CLI, policyName string, namespace string) error {
	if err := oc.AdminConfigClient().ConfigV1alpha1().ImagePolicies(namespace).Delete(context.TODO(), policyName, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete image policy %s in namespace %s: %v", policyName, namespace, err)
	}
	waitForPoolComplete(oc)
	return nil
}

func generateClusterImagePolicies() map[string]configv1alpha1.ClusterImagePolicy {
	testClusterImagePolicies := map[string]configv1alpha1.ClusterImagePolicy{
		invalidPublicKeyClusterImagePolicyName: {
			ObjectMeta: metav1.ObjectMeta{Name: invalidPublicKeyClusterImagePolicyName},
			Spec: configv1alpha1.ClusterImagePolicySpec{
				Scopes: []configv1alpha1.ImageScope{testReleaseImageScope},
				Policy: configv1alpha1.Policy{
					RootOfTrust: configv1alpha1.PolicyRootOfTrust{
						PolicyType: configv1alpha1.PublicKeyRootOfTrust,
						PublicKey: &configv1alpha1.PublicKey{
							KeyData: []byte(`LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFVW9GVW9ZQVJlS1hHeTU5eGU1U1FPazJhSjhvKwoyL1l6NVk4R2NOM3pGRTZWaUl2a0duSGhNbEFoWGFYL2JvME05UjYyczAvNnErK1Q3dXdORnVPZzhBPT0KLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0tCgo=`),
						},
					},
					SignedIdentity: configv1alpha1.PolicyIdentity{
						MatchPolicy: configv1alpha1.IdentityMatchPolicyMatchRepoDigestOrExact,
					},
				},
			},
		},
		publiKeyRekorClusterImagePolicyName: {
			ObjectMeta: metav1.ObjectMeta{Name: publiKeyRekorClusterImagePolicyName},
			Spec: configv1alpha1.ClusterImagePolicySpec{
				Scopes: []configv1alpha1.ImageScope{testReleaseImageScope},
				Policy: configv1alpha1.Policy{
					RootOfTrust: configv1alpha1.PolicyRootOfTrust{
						PolicyType: configv1alpha1.PublicKeyRootOfTrust,
						PublicKey: &configv1alpha1.PublicKey{
							KeyData: []byte(`-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA0ASyuH2TLWvBUqPHZ4Ip
75g7EncBkgQHdJnjzxAW5KQTMh/siBoB/BoSrtiPMwnChbTCnQOIQeZuDiFnhuJ7
M/D3b7JoX0m123NcCSn67mAdjBa6Bg6kukZgCP4ZUZeESajWX/EjylFcRFOXW57p
RDCEN42J/jYlVqt+g9+Grker8Sz86H3l0tbqOdjbz/VxHYhwF0ctUMHsyVRDq2QP
tqzNXlmlMhS/PoFr6R4u/7HCn/K+LegcO2fAFOb40KvKSKKVD6lewUZErhop1CgJ
XjDtGmmO9dGMF71mf6HEfaKSdy+EE6iSF2A2Vv9QhBawMiq2kOzEiLg4nAdJT8wg
ZrMAmPCqGIsXNGZ4/Q+YTwwlce3glqb5L9tfNozEdSR9N85DESfQLQEdY3CalwKM
BT1OEhEX1wHRCU4drMOej6BNW0VtscGtHmCrs74jPezhwNT8ypkyS+T0zT4Tsy6f
VXkJ8YSHyenSzMB2Op2bvsE3grY+s74WhG9UIA6DBxcTie15NSzKwfzaoNWODcLF
p7BY8aaHE2MqFxYFX+IbjpkQRfaeQQsouDFdCkXEFVfPpbD2dk6FleaMTPuyxtIT
gjVEtGQK2qGCFGiQHFd4hfV+eCA63Jro1z0zoBM5BbIIQ3+eVFwt3AlZp5UVwr6d
secqki/yrmv3Y0dqZ9VOn3UCAwEAAQ==
-----END PUBLIC KEY-----`),
							RekorKeyData: []byte(`-----BEGIN PUBLIC KEY-----
MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEDk0ElgGvMrsJULkg/ji1XX7EngDl2WY7
c75kKKy/SwWQ8n3Zymomy4DtkXzjsju204Mgjtdc7dVSPGSBn7VLLdDIzqSd1mLE
2ybPRzY8g742Mn/5hgH4eBzNKBjZ3wv1
-----END PUBLIC KEY-----`),
						},
					},
					SignedIdentity: configv1alpha1.PolicyIdentity{
						MatchPolicy: configv1alpha1.IdentityMatchPolicyMatchRepoDigestOrExact,
					},
				},
			},
		},
	}
	return testClusterImagePolicies
}

func generateImagePolicies() map[string]configv1alpha1.ImagePolicy {
	testImagePolicies := map[string]configv1alpha1.ImagePolicy{
		invalidPublicKeyImagePolicyName: {
			ObjectMeta: metav1.ObjectMeta{Name: invalidPublicKeyImagePolicyName},
			Spec: configv1alpha1.ImagePolicySpec{
				Scopes: []configv1alpha1.ImageScope{testReferenceImageScope},
				Policy: configv1alpha1.Policy{
					RootOfTrust: configv1alpha1.PolicyRootOfTrust{
						PolicyType: configv1alpha1.PublicKeyRootOfTrust,
						PublicKey: &configv1alpha1.PublicKey{
							KeyData: []byte(`LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFVW9GVW9ZQVJlS1hHeTU5eGU1U1FPazJhSjhvKwoyL1l6NVk4R2NOM3pGRTZWaUl2a0duSGhNbEFoWGFYL2JvME05UjYyczAvNnErK1Q3dXdORnVPZzhBPT0KLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0tCgo=`),
						},
					},
					SignedIdentity: configv1alpha1.PolicyIdentity{
						MatchPolicy: configv1alpha1.IdentityMatchPolicyMatchRepoDigestOrExact,
					},
				},
			},
		},
		publiKeyRekorImagePolicyName: {
			ObjectMeta: metav1.ObjectMeta{Name: publiKeyRekorImagePolicyName},
			Spec: configv1alpha1.ImagePolicySpec{
				Scopes: []configv1alpha1.ImageScope{testReferenceImageScope},
				Policy: configv1alpha1.Policy{
					RootOfTrust: configv1alpha1.PolicyRootOfTrust{
						PolicyType: configv1alpha1.PublicKeyRootOfTrust,
						PublicKey: &configv1alpha1.PublicKey{
							KeyData: []byte(`-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA0ASyuH2TLWvBUqPHZ4Ip
75g7EncBkgQHdJnjzxAW5KQTMh/siBoB/BoSrtiPMwnChbTCnQOIQeZuDiFnhuJ7
M/D3b7JoX0m123NcCSn67mAdjBa6Bg6kukZgCP4ZUZeESajWX/EjylFcRFOXW57p
RDCEN42J/jYlVqt+g9+Grker8Sz86H3l0tbqOdjbz/VxHYhwF0ctUMHsyVRDq2QP
tqzNXlmlMhS/PoFr6R4u/7HCn/K+LegcO2fAFOb40KvKSKKVD6lewUZErhop1CgJ
XjDtGmmO9dGMF71mf6HEfaKSdy+EE6iSF2A2Vv9QhBawMiq2kOzEiLg4nAdJT8wg
ZrMAmPCqGIsXNGZ4/Q+YTwwlce3glqb5L9tfNozEdSR9N85DESfQLQEdY3CalwKM
BT1OEhEX1wHRCU4drMOej6BNW0VtscGtHmCrs74jPezhwNT8ypkyS+T0zT4Tsy6f
VXkJ8YSHyenSzMB2Op2bvsE3grY+s74WhG9UIA6DBxcTie15NSzKwfzaoNWODcLF
p7BY8aaHE2MqFxYFX+IbjpkQRfaeQQsouDFdCkXEFVfPpbD2dk6FleaMTPuyxtIT
gjVEtGQK2qGCFGiQHFd4hfV+eCA63Jro1z0zoBM5BbIIQ3+eVFwt3AlZp5UVwr6d
secqki/yrmv3Y0dqZ9VOn3UCAwEAAQ==
-----END PUBLIC KEY-----`),
							RekorKeyData: []byte(`-----BEGIN PUBLIC KEY-----
MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEDk0ElgGvMrsJULkg/ji1XX7EngDl2WY7
c75kKKy/SwWQ8n3Zymomy4DtkXzjsju204Mgjtdc7dVSPGSBn7VLLdDIzqSd1mLE
2ybPRzY8g742Mn/5hgH4eBzNKBjZ3wv1
-----END PUBLIC KEY-----`),
						},
					},
					SignedIdentity: configv1alpha1.PolicyIdentity{
						MatchPolicy: configv1alpha1.IdentityMatchPolicyMatchRepoDigestOrExact,
					},
				},
			},
		},
	}
	return testImagePolicies
}
