package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8simage "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/openshift/library-go/pkg/build/naming"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	csiSecretsStoreDriver = "secrets-store.csi.k8s.io"
	testContainerName     = "test"
	podWaitTimeout        = 2 * time.Minute
)

var _ = g.Describe("[sig-storage][Feature:SecretsStore][Serial]", func() {
	defer g.GinkgoRecover()
	var (
		oc                     = exutil.NewCLIWithPodSecurityLevel("secrets-store-test-ns", admissionapi.LevelPrivileged)
		ctx                    = context.Background()
		baseDir                = exutil.FixturePath("testdata", "storage", "csi-secret-store")
		e2eSecretProviderClass = filepath.Join(baseDir, "e2e-provider-secretproviderclass.yaml")
	)

	g.BeforeEach(func() {
		// check if the CSIDriver exists, skip if not found
		_, err := oc.AdminKubeClient().StorageV1().CSIDrivers().Get(ctx, csiSecretsStoreDriver, metav1.GetOptions{})
		if err != nil {
			g.Skip(fmt.Sprintf("CSIDriver %s not found", csiSecretsStoreDriver))
		}

		// Allow creation of privileged pods for this test.
		// e2e-provider must be privileged to bind to a unix domain socket on the host.
		// The test pod must be privileged to read the files created by e2e-provider.
		g.By("adding privileged SCC to namespace")
		sa := fmt.Sprintf("system:serviceaccount:%s:default", oc.Namespace())
		err = oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "privileged", sa).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating e2e-provider SecretProviderClass")
		err = oc.AsAdmin().Run("apply").Args("-f", e2eSecretProviderClass).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodStates(oc)
		}
	})

	g.It("should allow pods to mount inline volumes from secret provider", func() {
		g.By("creating test pod with inline volume")
		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(ctx,
			getTestPodWithSecretsStore(oc.Namespace()),
			metav1.CreateOptions{},
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() { oc.KubeClient().CoreV1().Pods(oc.Namespace()).Delete(ctx, pod.Name, metav1.DeleteOptions{}) }()

		g.By("waiting for test pod to be ready")
		podNameLabel := exutil.ParseLabelsOrDie("name=" + pod.Name)
		pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), podNameLabel, exutil.CheckPodIsReady, 1, podWaitTimeout)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(pods)).To(o.Equal(1))

		g.By("checking pod log for secret value")
		output, err := oc.AsAdmin().Run("logs").WithoutNamespace().Args("pod/"+pod.Name, "-c", testContainerName, "-n", pod.Namespace).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).To(o.Equal("secret"))
	})
})

func getTestPodWithSecretsStore(namespace string) *corev1.Pod {
	podName := naming.GetPodName("test-pod", uuid.New().String())
	privileged := true
	ro := true
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    map[string]string{"name": podName},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "default",
			Containers: []corev1.Container{
				{
					Name:    testContainerName,
					Image:   k8simage.GetE2EImage(k8simage.BusyBox),
					Command: []string{"sh", "-c", "cat /mnt/test-vol/foo && sleep 120"},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &privileged,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "test-vol",
							MountPath: "/mnt/test-vol",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "test-vol",
					VolumeSource: corev1.VolumeSource{
						CSI: &corev1.CSIVolumeSource{
							Driver:           csiSecretsStoreDriver,
							ReadOnly:         &ro,
							VolumeAttributes: map[string]string{"secretProviderClass": "e2e-provider"},
						},
					},
				},
			},
		},
	}
	return pod
}
