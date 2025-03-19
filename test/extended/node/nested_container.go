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

var (
	oc          = exutil.NewCLIWithPodSecurityLevel("nested-podman", admissionapi.LevelBaseline)
	customImage = exutil.FixturePath("testdata", "node", "nested_container")
	name        = "baseline-nested-container"
)

var _ = g.Describe("[Suite:openshift/usernamespace] [sig-node] [FeatureGate:ProcMountType] [FeatureGate:UserNamespacesSupport] nested container", func() {
	g.DescribeTable("should pass podman localsystem test in baseline mode",
		func(ctx context.Context, batsFile string) {
			if !exutil.IsTechPreviewNoUpgrade(ctx, oc.AdminConfigClient()) {
				g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
			}
			runNestedPod(ctx, batsFile)
		},
		g.Entry(nil, "001-basic.bats"),
		g.Entry(nil, "005-info.bats"),
		g.Entry(nil, "010-images.bats"),
		g.Entry(nil, "011-image.bats"),
		g.Entry(nil, "012-manifest.bats"),
		g.Entry(nil, "015-help.bats"),
		g.Entry(nil, "020-tag.bats"),
		g.Entry(nil, "030-run.bats"),
		g.Entry(nil, "032-sig-proxy.bats"),
		g.Entry(nil, "035-logs.bats"),
		g.Entry(nil, "037-runlabel.bats"),
		g.Entry(nil, "040-ps.bats"),
		g.Entry(nil, "045-start.bats"),
		g.Entry(nil, "050-stop.bats"),
		g.Entry(nil, "055-rm.bats"),
		g.Entry(nil, "060-mount.bats"),
		g.Entry(nil, "065-cp.bats"),
		g.Entry(nil, "070-build.bats"),
		g.Entry(nil, "075-exec.bats"),
		// g.Entry(nil, "080-pause.bats"),
		g.Entry(nil, "085-top.bats"),
		g.Entry(nil, "090-events.bats"),
		g.Entry(nil, "110-history.bats"),
		g.Entry(nil, "120-load.bats"),
		g.Entry(nil, "125-import.bats"),
		g.Entry(nil, "130-kill.bats"),
		g.Entry(nil, "140-diff.bats"),
		g.Entry(nil, "150-login.bats"),
		g.Entry(nil, "155-partial-pull.bats"),
		g.Entry(nil, "160-volumes.bats"),
		g.Entry(nil, "170-run-userns.bats"),
		g.Entry(nil, "180-blkio.bats"),
		g.Entry(nil, "190-run-ipcns.bats"),
		g.Entry(nil, "195-run-namespaces.bats"),
		g.Entry(nil, "200-pod.bats"),
		g.Entry(nil, "220-healthcheck.bats"),
		// g.Entry(nil, "250-systemd.bats"),
		// g.Entry(nil, "251-system-service.bats"),
		// g.Entry(nil, "252-quadlet.bats"),
		// g.Entry(nil, "255-auto-update.bats"),
		g.Entry(nil, "260-sdnotify.bats"),
		// g.Entry(nil, "270-socket-activation.bats"),
		g.Entry(nil, "271-tcp-cors-server.bats"),
		g.Entry(nil, "272-system-connection.bats"),
		g.Entry(nil, "280-update.bats"),
		g.Entry(nil, "300-cli-parsing.bats"),
		g.Entry(nil, "320-system-df.bats"),
		g.Entry(nil, "330-corrupt-images.bats"),
		g.Entry(nil, "331-system-check.bats"),
		g.Entry(nil, "400-unprivileged-access.bats"),
		g.Entry(nil, "410-selinux.bats"),
		g.Entry(nil, "420-cgroups.bats"),
		g.Entry(nil, "450-interactive.bats"),
		g.Entry(nil, "500-networking.bats"),
		g.Entry(nil, "505-networking-pasta.bats"),
		g.Entry(nil, "520-checkpoint.bats"),
		g.Entry(nil, "550-pause-process.bats"),
		g.Entry(nil, "600-completion.bats"),
		g.Entry(nil, "610-format.bats"),
		g.Entry(nil, "620-option-conflicts.bats"),
		g.Entry(nil, "700-play.bats"),
		g.Entry(nil, "710-kube.bats"),
		g.Entry(nil, "750-trust.bats"),
		g.Entry(nil, "760-system-renumber.bats"),
		g.Entry(nil, "800-config.bats"),
		g.Entry(nil, "850-compose.bats"),
		g.Entry(nil, "900-ssh.bats"),
		g.Entry(nil, "950-preexec-hooks.bats"),
		// g.Entry(nil, "999-final.bats"), // it doesn't actually run tests
	)
})

func runNestedPod(ctx context.Context, testFile string) {
	//g.By("create custom builder image")
	//err := oc.Run("new-build").Args("--binary", "--strategy=docker", fmt.Sprintf("--name=%s", name)).Execute()
	//o.Expect(err).NotTo(o.HaveOccurred())
	//br, _ := exutil.StartBuildAndWait(oc, name, fmt.Sprintf("--from-dir=%s", customImage))
	//br.AssertSuccess()
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
					Image:           "quay.io/crio/nested-container:v5.4.0",
					ImagePullPolicy: corev1.PullAlways,
					Args: []string{
						"sh", "-c",
						fmt.Sprintf("PODMAN=$(pwd)/bin/podman bats -T --filter-tags '!ci:parallel' test/system/%s && PODMAN=$(pwd)/bin/podman bats -T --filter-tags 'ci:parallel' -j $(nproc) test/system/%s",
							testFile, testFile),
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
	var logs []byte
	o.Eventually(func() error {
		pod, err := oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			return fmt.Errorf("pod %s is not in a terminal state: %s", name, pod.Status.Phase)
		}
		logs, err = oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{}).Do(ctx).Raw()
		return err
	}, "10m", "10s").Should(o.Succeed(), fmt.Sprintf("podman system test timedout:\n%s", logs))

	pod, err = oc.AsAdmin().KubeClient().CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(pod.Status.Phase).To(o.Equal(corev1.PodSucceeded), fmt.Sprintf("podman system test failed:\n%s", logs))
}
