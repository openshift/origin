package security

import (
	"context"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"

	g "github.com/onsi/ginkgo"
	t "github.com/onsi/ginkgo/extensions/table"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-arch] [Conformance] sysctl", func() {
	oc := exutil.NewCLI("sysctl")
	t.DescribeTable("pod should start for each sysctl on whitelist", func(sysctl, value, path string) {
		f := oc.KubeFramework()
		tuningTestPod := frameworkpod.CreateExecPodOrFail(f.ClientSet, f.Namespace.Name, "sysctl-pod-", func(pod *v1.Pod) {
			pod.Spec.SecurityContext.Sysctls = []v1.Sysctl{
				{
					Name:  sysctl,
					Value: value,
				},
			}
		})

		g.By("checking that the sysctl was set")
		output, err := oc.AsAdmin().Run("exec").Args(tuningTestPod.Name, "--", "/bin/bash", "-c", "cat "+path).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")
		o.Expect(output).Should(o.Equal(value))

	},
		t.Entry("kernel.shm_rmid_forced", "kernel.shm_rmid_forced", "1", "/proc/sys/kernel/shm_rmid_forced"),
		t.Entry("net.ipv4.ip_local_port_range", "net.ipv4.ip_local_port_range", "32768\t61000", "/proc/sys/net/ipv4/ip_local_port_range"),
		t.Entry("net.ipv4.tcp_syncookies", "net.ipv4.tcp_syncookies", "1", "/proc/sys/net/ipv4/tcp_syncookies"),
		t.Entry("net.ipv4.ping_group_range", "net.ipv4.ping_group_range", "1\t0", "/proc/sys/net/ipv4/ping_group_range"),
		t.Entry("net.ipv4.ip_unprivileged_port_start", "net.ipv4.ip_unprivileged_port_start", "1002", "/proc/sys/net/ipv4/ip_unprivileged_port_start"),
	)

	t.DescribeTable("pod should not start for sysctl not on whitelist", func(sysctl, value string) {
		f := oc.KubeFramework()
		podDefinition := frameworkpod.NewAgnhostPod(f.Namespace.Name, "sysctl-pod", nil, nil, nil)
		podDefinition.Spec.SecurityContext.Sysctls = []v1.Sysctl{{Name: sysctl, Value: value}}
		execPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.TODO(), podDefinition, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "error creating pod")
		err = wait.PollImmediate(2*time.Second, 1*time.Minute, func() (bool, error) {
			retrievedPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), execPod.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			return retrievedPod.Status.Phase == v1.PodFailed, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not be able to create pod")
	},
		t.Entry("kernel.msgmax", "kernel.msgmax", "1000"),
		t.Entry("net.ipv4.ip_dynaddr", "net.ipv4.ip_dynaddr", "1"),
	)
})
