package security

import (
	g "github.com/onsi/ginkgo"
	t "github.com/onsi/ginkgo/extensions/table"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
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
})
