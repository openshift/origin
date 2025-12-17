package security

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-arch] [Conformance] sysctl", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("sysctl", admissionapi.LevelPrivileged)
	ctx := context.Background()
	g.DescribeTable("whitelists", g.Label("Size:M"), func(sysctl, value, path, defaultSysctlValue string) {
		f := oc.KubeFramework()
		var preexistingPod *v1.Pod
		var err error
		var nodeOutputBeforeSysctlAplied, previousPodSysctlValue string

		g.By("creating a preexisting pod to validate sysctl are not applied on it and on the node", func() {
			preexistingPod = frameworkpod.CreateExecPodOrFail(ctx, f.ClientSet, f.Namespace.Name, "sysctl-pod-", func(pod *v1.Pod) {
				pod.Spec.Volumes = []v1.Volume{
					{Name: "sysvolume", VolumeSource: v1.VolumeSource{HostPath: &v1.HostPathVolumeSource{Path: "/proc"}}},
				}
				pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{{Name: "sysvolume", MountPath: "/host/proc"}}
			})
			nodeOutputBeforeSysctlAplied, err = oc.AsAdmin().Run("exec").Args(preexistingPod.Name, "--", "cat", "/host/"+path).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")
			previousPodSysctlValue, err = oc.AsAdmin().Run("exec").Args(preexistingPod.Name, "--", "cat", path).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")

			// Retrieve created pod so we can use the same NodeName
			preexistingPod, err = f.ClientSet.CoreV1().Pods(preexistingPod.Namespace).Get(ctx, preexistingPod.Name, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "unable get running pod")
			o.Expect(preexistingPod.Spec.NodeName).NotTo(o.BeEmpty(), "expected scheduled pod but found empty Spec.NodeName")
		})

		g.By("creating a pod with a sysctl", func() {
			tuningTestPod := frameworkpod.CreateExecPodOrFail(ctx, f.ClientSet, f.Namespace.Name, "sysctl-pod-", func(pod *v1.Pod) {
				pod.Spec.SecurityContext.Sysctls = []v1.Sysctl{{Name: sysctl, Value: value}}
				pod.Spec.NodeName = preexistingPod.Spec.NodeName
			})
			g.By("checking that the sysctl was set")
			output, err := oc.AsAdmin().Run("exec").Args(tuningTestPod.Name, "--", "cat", path).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")
			o.Expect(output).Should(o.Equal(value))
		})

		g.By("checking node sysctl did not change", func() {
			nodeOutputAfterSysctlAplied, err := oc.AsAdmin().Run("exec").Args(preexistingPod.Name, "--", "cat", "/host/"+path).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")
			o.Expect(nodeOutputBeforeSysctlAplied).Should(o.Equal(nodeOutputAfterSysctlAplied))
		})

		g.By("checking sysctl on preexising pod did not change", func() {
			podOutputAfterSysctlAplied, err := oc.AsAdmin().Run("exec").Args(preexistingPod.Name, "--", "cat", path).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")
			o.Expect(previousPodSysctlValue).Should(o.Equal(podOutputAfterSysctlAplied))
		})

		g.By("checking that sysctls of new pods are not affected", func() {
			nextPod := frameworkpod.CreateExecPodOrFail(ctx, f.ClientSet, f.Namespace.Name, "sysctl-pod-", func(pod *v1.Pod) {
				pod.Spec.NodeName = preexistingPod.Spec.NodeName
			})
			podOutput, err := oc.AsAdmin().Run("exec").Args(nextPod.Name, "--", "cat", path).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")
			o.Expect(podOutput).Should(o.Equal(defaultSysctlValue))
		})
	},
		g.Entry("kernel.shm_rmid_forced", "kernel.shm_rmid_forced", "1", "/proc/sys/kernel/shm_rmid_forced", "0"),
		g.Entry("net.ipv4.ip_local_port_range", "net.ipv4.ip_local_port_range", "32769\t61001", "/proc/sys/net/ipv4/ip_local_port_range", "32768\t60999"),
		g.Entry("net.ipv4.tcp_syncookies", "net.ipv4.tcp_syncookies", "0", "/proc/sys/net/ipv4/tcp_syncookies", "1"),
		g.Entry("net.ipv4.ping_group_range", "net.ipv4.ping_group_range", "1\t0", "/proc/sys/net/ipv4/ping_group_range", "0\t2147483647"),
		g.Entry("net.ipv4.ip_unprivileged_port_start", "net.ipv4.ip_unprivileged_port_start", "1002", "/proc/sys/net/ipv4/ip_unprivileged_port_start", "1024"),
	)

	g.DescribeTable("pod should not start for sysctl not on whitelist", g.Label("Size:S"), func(sysctl, value string) {
		f := oc.KubeFramework()
		podDefinition := frameworkpod.NewAgnhostPod(f.Namespace.Name, "sysctl-pod", nil, nil, nil)
		podDefinition.Spec.SecurityContext.Sysctls = []v1.Sysctl{{Name: sysctl, Value: value}}
		execPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, podDefinition, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "error creating pod")
		err = wait.PollImmediate(2*time.Second, 1*time.Minute, func() (bool, error) {
			retrievedPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(ctx, execPod.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			return retrievedPod.Status.Phase == v1.PodFailed, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not be able to create pod")
	},
		g.Entry("kernel.msgmax", "kernel.msgmax", "1000"),
		g.Entry("net.ipv4.ip_dynaddr", "net.ipv4.ip_dynaddr", "1"),
	)
})
