package node

import (
	"context"
	"fmt"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
)

var oc = exutil.NewCLIWithPodSecurityLevel("nested-podman", admissionapi.LevelBaseline)
var name = "baseline-nested-container"

var _ = g.Describe("[sig-node][FeatureGate:ProcMountType][FeatureGate:UserNamespacesSupport] nested container", func() {
	g.It("should pass podman localsystem test in baseline mode (serial)", func(ctx context.Context) {
		if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
			g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
		}
		runNestedPod(ctx, "sh", "-c", "PODMAN=$(pwd)/bin/podman bats -T --filter-tags '!ci:parallel' test/system/")
	})

	g.It("should pass podman localsystem test in baseline mode (parallel)", func(ctx context.Context) {
		if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
			g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
		}
		runNestedPod(ctx, "sh", "-c", "PODMAN=$(pwd)/bin/podman bats -T --filter-tags 'ci:parallel' -j $(nproc) test/system/")
	})

})

func runNestedPod(ctx context.Context, command ...string) {
	// Don't build the image but use the prebuilt image, because it takes a lot of time and causes interruption.
	g.By("creating a pod with a nested container")
	namespace := oc.Namespace()
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"io.kubernetes.cri-o.Devices": "/dev/fuse,/dev/net/tun",
			},
		},
		Spec: corev1.PodSpec{
			HostUsers: pointer.Bool(false),
			DNSPolicy: corev1.DNSNone,
			DNSConfig: &corev1.PodDNSConfig{
				Nameservers: []string{"1.1.1.1"},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "nested-podman",
					Image:           fmt.Sprintf("quay.io/crio/nested-container:v5.4.0"),
					ImagePullPolicy: corev1.PullAlways,
					Args:            command,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: pointer.Int64(1000),
						ProcMount: ptr.To(corev1.UnmaskedProcMount),
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"SETUID",
								"SETGID",
							},
						},
						SELinuxOptions: &corev1.SELinuxOptions{
							Type: "container_engine_t",
						},
					},
				},
			},
		},
	}
	_, err := oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("waiting for the pod to complete")
	o.Eventually(func() error {
		pod, err := oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			return fmt.Errorf("pod %s is not in a terminal state: %s", name, pod.Status.Phase)
		}
		return nil
	}, "20m", "10s").Should(o.Succeed())

	g.By("fetching the logs from the pod and checking for errors")
	logs, err := oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{}).Do(ctx).Raw()
	o.Expect(err).NotTo(o.HaveOccurred())

	pod, err = oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pod.Status.Phase).To(o.Equal(corev1.PodSucceeded), fmt.Sprintf("podman system test failed:\n%s", logs))
}
