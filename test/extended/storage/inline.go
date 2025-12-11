package storage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8simage "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	csiInlineVolProfileLabel = "security.openshift.io/csi-ephemeral-volume-profile"
	csiTestDriver            = "csi.test.openshift.io"
	podSecurityEnforceError  = "has a pod security enforce level that is lower than"
)

// This is [Serial] because it modifies a CSIDriver object that is used by multiple tests.
var _ = g.Describe("[sig-storage][Feature:CSIInlineVolumeAdmission][Serial]", func() {
	defer g.GinkgoRecover()
	var (
		ctx = context.Background()

		beforeEach = func(oc *exutil.CLI) {
			exutil.PreTestDump()

			// Since this only tests the admission plugin for inline volumes,
			// we don't actually need to deploy any CSI driver pods.
			// We just need a CSIDriver object for this test.
			g.By("creating CSI driver for inline volumes")
			_, err := oc.AdminKubeClient().StorageV1().CSIDrivers().Create(ctx,
				getTestCSIDriver(),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		afterEach = func(oc *exutil.CLI) {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
			}

			g.By("deleting CSI driver for inline volumes")
			err := oc.AdminKubeClient().StorageV1().CSIDrivers().Delete(ctx,
				csiTestDriver,
				metav1.DeleteOptions{},
			)
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

		g.It("should allow pods with inline volumes when the driver uses the privileged label", g.Label("Size:M"), func() {
			g.By("setting the csi-ephemeral-volume-profile label to privileged")
			err := setCSIEphemeralVolumeProfile(ctx, oc, "privileged")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test pod with inline volume")
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx,
				getTestPodWithInlineVol(oc.Namespace()),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()
		})

		g.It("should allow pods with inline volumes when the driver uses the restricted label", g.Label("Size:M"), func() {
			g.By("setting the csi-ephemeral-volume-profile label to restricted")
			err := setCSIEphemeralVolumeProfile(ctx, oc, "restricted")
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

		g.It("should deny pods with inline volumes when the driver uses the privileged label", g.Label("Size:M"), func() {
			g.By("setting the csi-ephemeral-volume-profile label to privileged")
			err := setCSIEphemeralVolumeProfile(ctx, oc, "privileged")
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

		g.It("should allow pods with inline volumes when the driver uses the baseline label", g.Label("Size:M"), func() {
			g.By("setting the csi-ephemeral-volume-profile label to baseline")
			err := setCSIEphemeralVolumeProfile(ctx, oc, "baseline")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test pod with inline volume")
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx,
				getTestPodWithInlineVol(oc.Namespace()),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()
		})

		g.It("should allow pods with inline volumes when the driver uses the restricted label", g.Label("Size:M"), func() {
			g.By("setting the csi-ephemeral-volume-profile label to restricted")
			err := setCSIEphemeralVolumeProfile(ctx, oc, "restricted")
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

		g.It("should deny pods with inline volumes when the driver uses the privileged label", g.Label("Size:M"), func() {
			g.By("setting the csi-ephemeral-volume-profile label to privileged")
			err := setCSIEphemeralVolumeProfile(ctx, oc, "privileged")
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

		g.It("should deny pods with inline volumes when the driver uses the baseline label", g.Label("Size:M"), func() {
			g.By("setting the csi-ephemeral-volume-profile label to baseline")
			err := setCSIEphemeralVolumeProfile(ctx, oc, "baseline")
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

		g.It("should allow pods with inline volumes when the driver uses the restricted label", g.Label("Size:M"), func() {
			g.By("setting the csi-ephemeral-volume-profile label to restricted")
			err := setCSIEphemeralVolumeProfile(ctx, oc, "restricted")
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

// setCSIEphemeralVolumeProfile sets the security.openshift.io/csi-ephemeral-volume-profile
// label to the provided value on the csi.test.openshift.io CSIDriver object.
func setCSIEphemeralVolumeProfile(ctx context.Context, oc *exutil.CLI, labelValue string) error {
	patch := []byte(fmt.Sprintf(`{"metadata": {"labels":{"%s": "%s"}}}`, csiInlineVolProfileLabel, labelValue))
	_, err := oc.AdminKubeClient().StorageV1().CSIDrivers().Patch(ctx, csiTestDriver, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	return err
}

func getTestCSIDriver() *storagev1.CSIDriver {
	driver := &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name:   csiTestDriver,
			Labels: map[string]string{csiInlineVolProfileLabel: "restricted"},
		},
		Spec: storagev1.CSIDriverSpec{
			VolumeLifecycleModes: []storagev1.VolumeLifecycleMode{
				storagev1.VolumeLifecycleEphemeral,
			},
		},
	}
	return driver
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
	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: "test-vol",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					Driver: csiTestDriver,
				},
			},
		},
	}
	return pod
}
