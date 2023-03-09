package storage

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8simage "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	csiInlineVolProfileLabel = "security.openshift.io/csi-ephemeral-volume-profile"
	csiSharedResourceDriver  = "csi.sharedresource.openshift.io"
	podSecurityEnforceError  = "has a pod security enforce level that is lower than"
)

// This is [Serial] because it modifies a CSIDriver object that is used by multiple tests.
var _ = g.Describe("[sig-storage][Feature:CSIInlineVolumeAdmission][Serial]", func() {
	defer g.GinkgoRecover()
	var (
		ctx                  = context.Background()
		baseDir              = exutil.FixturePath("testdata", "storage", "inline")
		secret               = filepath.Join(baseDir, "secret.yaml")
		csiSharedSecret      = filepath.Join(baseDir, "csi-sharedsecret.yaml")
		csiSharedRole        = filepath.Join(baseDir, "csi-sharedresourcerole.yaml")
		csiSharedRoleBinding = filepath.Join(baseDir, "csi-sharedresourcerolebinding.yaml")

		beforeEach = func(oc *exutil.CLI) {
			// TODO: remove this after the shared resource driver is GA
			if !exutil.IsTechPreviewNoUpgrade(oc) {
				g.Skip("this test is only expected to work with TechPreviewNoUpgrade clusters")
			}
			exutil.PreTestDump()

			// create the secret to share in a new namespace
			g.By("creating a secret")
			err := oc.AsAdmin().Run("--namespace=default", "apply").Args("-f", secret).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// create the csi shared secret object
			g.By("creating a csi shared secret resource")
			err = oc.AsAdmin().Run("apply").Args("-f", csiSharedSecret).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// process the role to grant use of the share
			g.By("creating a csi shared role resource")
			err = oc.AsAdmin().Run("apply").Args("-f", csiSharedRole).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// process the rolebinding to grant use of the share
			g.By("creating a csi shared role binding resource")
			rolebinding, _, err := oc.AsAdmin().Run("process").Args("-f", csiSharedRoleBinding, "-p", fmt.Sprintf("NAMESPACE=%s", oc.Namespace())).Outputs()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().Run("apply").Args("-f", "-").InputString(rolebinding).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		afterEach = func(oc *exutil.CLI) {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
			}

			// set this back to the default value at the end of each test
			g.By("setting the csi-ephemeral-volume-profile label back to restricted")
			err := setCSIEphemeralVolumeProfile(oc, "restricted")
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	)

	g.Context("privileged namespace", func() {
		var (
			oc = exutil.NewCLIWithPodSecurityLevel("inline-vol-privileged-ns", admissionapi.LevelPrivileged)
		)

		g.BeforeEach(func() {
			beforeEach(oc)
		})

		g.AfterEach(func() {
			afterEach(oc)
		})

		g.It("should allow pods with inline volumes when the driver uses the privileged label", func() {
			g.By("setting the csi-ephemeral-volume-profile label to privileged")
			err := setCSIEphemeralVolumeProfile(oc, "privileged")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test pod with inline volume")
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx,
				getTestPodWithInlineVol(oc.Namespace()),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()
		})

		g.It("should allow pods with inline volumes when the driver uses the restricted label", func() {
			g.By("setting the csi-ephemeral-volume-profile label to restricted")
			err := setCSIEphemeralVolumeProfile(oc, "restricted")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test pod with inline volume")
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx,
				getTestPodWithInlineVol(oc.Namespace()),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()
		})
	})

	g.Context("baseline namespace", func() {
		var (
			oc = exutil.NewCLIWithPodSecurityLevel("inline-vol-baseline-ns", admissionapi.LevelBaseline)
		)

		g.BeforeEach(func() {
			beforeEach(oc)
		})

		g.AfterEach(func() {
			afterEach(oc)
		})

		g.It("should deny pods with inline volumes when the driver uses the privileged label", func() {
			g.By("setting the csi-ephemeral-volume-profile label to privileged")
			err := setCSIEphemeralVolumeProfile(oc, "privileged")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test pod with inline volume")
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx,
				getTestPodWithInlineVol(oc.Namespace()),
				metav1.CreateOptions{},
			)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring(podSecurityEnforceError))
			defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()
		})

		g.It("should allow pods with inline volumes when the driver uses the baseline label", func() {
			g.By("setting the csi-ephemeral-volume-profile label to baseline")
			err := setCSIEphemeralVolumeProfile(oc, "baseline")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test pod with inline volume")
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx,
				getTestPodWithInlineVol(oc.Namespace()),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()
		})

		g.It("should allow pods with inline volumes when the driver uses the restricted label", func() {
			g.By("setting the csi-ephemeral-volume-profile label to restricted")
			err := setCSIEphemeralVolumeProfile(oc, "restricted")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test pod with inline volume")
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx,
				getTestPodWithInlineVol(oc.Namespace()),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()
		})
	})

	g.Context("restricted namespace", func() {
		var (
			oc = exutil.NewCLIWithPodSecurityLevel("inline-vol-restricted-ns", admissionapi.LevelRestricted)
		)

		g.BeforeEach(func() {
			beforeEach(oc)
		})

		g.AfterEach(func() {
			afterEach(oc)
		})

		g.It("should deny pods with inline volumes when the driver uses the privileged label", func() {
			g.By("setting the csi-ephemeral-volume-profile label to privileged")
			err := setCSIEphemeralVolumeProfile(oc, "privileged")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test pod with inline volume")
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx,
				getTestPodWithInlineVol(oc.Namespace()),
				metav1.CreateOptions{},
			)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring(podSecurityEnforceError))
			defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()
		})

		g.It("should deny pods with inline volumes when the driver uses the baseline label", func() {
			g.By("setting the csi-ephemeral-volume-profile label to baseline")
			err := setCSIEphemeralVolumeProfile(oc, "baseline")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test pod with inline volume")
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx,
				getTestPodWithInlineVol(oc.Namespace()),
				metav1.CreateOptions{},
			)
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring(podSecurityEnforceError))
			defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()
		})

		g.It("should allow pods with inline volumes when the driver uses the restricted label", func() {
			g.By("setting the csi-ephemeral-volume-profile label to restricted")
			err := setCSIEphemeralVolumeProfile(oc, "restricted")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test pod with inline volume")
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx,
				getTestPodWithInlineVol(oc.Namespace()),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()
		})
	})
})

// setCSIEphemeralVolumeProfile sets the security.openshift.io/csi-ephemeral-volume-profile label to the provided
// value on the csi.sharedresource.openshift.io CSIDriver object.
func setCSIEphemeralVolumeProfile(oc *exutil.CLI, labelValue string) error {
	label := fmt.Sprintf("%s=%s", csiInlineVolProfileLabel, labelValue)
	return oc.AsAdmin().Run("label").Args("--overwrite", "csidriver", csiSharedResourceDriver, label).Execute()
}

func getTestPod(namespace string) *corev1.Pod {
	runAsNonRoot := true
	allowPrivEsc := false
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-" + uuid.New().String(),
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "default",
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
			},
			Containers: []corev1.Container{
				{
					Name:    "test",
					Image:   k8simage.GetE2EImage(k8simage.BusyBox),
					Command: []string{"/bin/true"},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: &allowPrivEsc,
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
					},
				},
			},
		},
	}
	return pod
}

func getTestPodWithInlineVol(namespace string) *corev1.Pod {
	pod := getTestPod(namespace)
	ro := true
	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: "test-vol",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver:           csiSharedResourceDriver,
					ReadOnly:         &ro,
					VolumeAttributes: map[string]string{"sharedSecret": "my-share"},
				},
			},
		},
	}
	return pod
}
