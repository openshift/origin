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
	"os"
)

var (
	oc   = exutil.NewCLIWithPodSecurityLevel("nested-podman", admissionapi.LevelBaseline)
	name = "baseline-nested-container"
)

var _ = g.Describe("[sig-node] [FeatureGate:ProcMountType] [FeatureGate:UserNamespacesSupport] nested container", func() {
	g.It("should pass podman localsystem test in baseline mode",
		func(ctx context.Context) {
			if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
				g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
			}
			runNestedPod(ctx)
		},
	)
})

func runNestedPod(ctx context.Context) {
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
					Image:           "quay.io/rh-ee-atokubi/nested-podman",
					ImagePullPolicy: corev1.PullAlways,
					Args: []string{
						"./run_tests.sh",
					},
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
		_, err := oc.AsAdmin().Run("exec").Args(pod.Name, "--", "[", "-f", "done", "]").Output()
		if err != nil {
			return err
		}
		return nil
	}, "30m", "10s").Should(o.Succeed())
	o.Expect(err).NotTo(o.HaveOccurred())

	artifact := os.Getenv("ARTIFACT_DIR")
	g.By(fmt.Sprintf("Fetching artifact to %s", artifact))
	_, err = oc.AsAdmin().Run("cp").Args(
		fmt.Sprintf("%s:junit/serial-nested-container.xml", pod.Name),
		fmt.Sprintf("%s/junit/serial-nested-container.xml", artifact),
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = oc.AsAdmin().Run("cp").Args(
		fmt.Sprintf("%s:junit/parallel-nested-container.xml", pod.Name),
		fmt.Sprintf("%s/junit/parallel-nested-container.xml", artifact),
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
}
