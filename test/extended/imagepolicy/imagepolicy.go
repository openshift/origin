package imagepolicy

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	machineconfighelper "github.com/openshift/origin/test/extended/machine_config"
	exutil "github.com/openshift/origin/test/extended/util"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	testReleaseImageScope             = "quay.io/openshift-release-dev/ocp-release@sha256:fbad931c725b2e5b937b295b58345334322bdabb0b67da1c800a53686d7397da"
	testReferenceImageScope           = "quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:4db234f37ae6712e2f7ed8d13f7fb49971c173d0e4f74613d0121672fa2e01f5"
	registriesWorkerPoolMachineConfig = "99-worker-generated-registries"
	registriesMasterPoolMachineConfig = "99-master-generated-registries"
	testPodName                       = "signature-validation-test-pod"
	workerPool                        = "worker"
	masterPool                        = "master"
	SignatureValidationFaildReason    = "SignatureValidationFailed"
)

var _ = g.Describe("[sig-imagepolicy][OCPFeatureGate:SigstoreImageVerification][Serial]", g.Ordered, func() {
	defer g.GinkgoRecover()
	var (
		oc                                        = exutil.NewCLIWithoutNamespace("cluster-image-policy")
		tctx                                      = context.Background()
		cli                                       = exutil.NewCLIWithPodSecurityLevel("verifysigstore-e2e", admissionapi.LevelBaseline)
		clif                                      = cli.KubeFramework()
		imgpolicyCli                              = exutil.NewCLIWithPodSecurityLevel("verifysigstore-imagepolicy-e2e", admissionapi.LevelBaseline)
		imgpolicyClif                             = imgpolicyCli.KubeFramework()
		imagePolicyBaseDir                        = exutil.FixturePath("testdata", "imagepolicy")
		invalidPublicKeyClusterImagePolicyFixture = filepath.Join(imagePolicyBaseDir, "invalid-public-key-cluster-image-policy.yaml")
		publiKeyRekorClusterImagePolicyFixture    = filepath.Join(imagePolicyBaseDir, "public-key-rekor-cluster-image-policy.yaml")
		invalidPublicKeyImagePolicyFixture        = filepath.Join(imagePolicyBaseDir, "invalid-public-key-image-policy.yaml")
		publiKeyRekorImagePolicyFixture           = filepath.Join(imagePolicyBaseDir, "public-key-rekor-image-policy.yaml")
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
		createClusterImagePolicy(oc, invalidPublicKeyClusterImagePolicyFixture)
		g.DeferCleanup(deleteClusterImagePolicy, oc, invalidPublicKeyClusterImagePolicyFixture)

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

		createClusterImagePolicy(oc, invalidPublicKeyClusterImagePolicyFixture)
		g.DeferCleanup(deleteClusterImagePolicy, oc, invalidPublicKeyClusterImagePolicyFixture)

		waitForPoolComplete(oc)

		pod, err := launchTestPod(tctx, clif, testPodName, testReleaseImageScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = waitForTestPodContainerToFailSignatureValidation(tctx, clif, pod)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should pass clusterimagepolicy signature validation with signed image", func() {
		createClusterImagePolicy(oc, publiKeyRekorClusterImagePolicyFixture)
		g.DeferCleanup(deleteClusterImagePolicy, oc, publiKeyRekorClusterImagePolicyFixture)

		waitForPoolComplete(oc)

		pod, err := launchTestPod(tctx, clif, testPodName, testReleaseImageScope)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(deleteTestPod, tctx, clif, testPodName)

		err = e2epod.WaitForPodSuccessInNamespace(tctx, clif.ClientSet, pod.Name, pod.Namespace)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("Should fail imagepolicy signature validation in different namespaces root of trust does not match the identity in the signature", func() {
		createImagePolicy(oc, invalidPublicKeyImagePolicyFixture, imgpolicyClif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, invalidPublicKeyImagePolicyFixture, imgpolicyClif.Namespace.Name)

		createImagePolicy(oc, invalidPublicKeyImagePolicyFixture, clif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, invalidPublicKeyImagePolicyFixture, clif.Namespace.Name)

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
		createImagePolicy(oc, publiKeyRekorImagePolicyFixture, clif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, publiKeyRekorImagePolicyFixture, clif.Namespace.Name)

		createImagePolicy(oc, publiKeyRekorImagePolicyFixture, imgpolicyClif.Namespace.Name)
		g.DeferCleanup(deleteImagePolicy, oc, publiKeyRekorImagePolicyFixture, imgpolicyClif.Namespace.Name)

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

func createClusterImagePolicy(oc *exutil.CLI, fixture string) {
	err := oc.Run("create").Args("-f", fixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func deleteClusterImagePolicy(oc *exutil.CLI, fixture string) error {
	return oc.Run("delete").Args("-f", fixture).Execute()
}

func createImagePolicy(oc *exutil.CLI, fixture string, namespace string) {
	err := oc.Run("create").Args("-f", fixture, "-n", namespace).Execute()
	o.Expect(err).NotTo(o.HaveOccurred())
}

func waitForPoolComplete(oc *exutil.CLI) {
	time.Sleep(10 * time.Second)
	machineconfighelper.WaitForConfigAndPoolComplete(oc, workerPool, registriesWorkerPoolMachineConfig)
	machineconfighelper.WaitForConfigAndPoolComplete(oc, masterPool, registriesMasterPoolMachineConfig)
}

func deleteImagePolicy(oc *exutil.CLI, fixture string, namespace string) error {
	return oc.Run("delete").Args("-f", fixture, "-n", namespace).Execute()
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
