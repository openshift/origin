package networking

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-network] external gateway address", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("ns-global", admissionapi.LevelPrivileged)

	InOVNKubernetesContext(func() {
		f := oc.KubeFramework()

		g.It("should match the address family of the pod", func() {
			podIPFamily := GetIPFamilyForCluster(f)
			o.Expect(podIPFamily).NotTo(o.Equal(Unknown))
			// Set external gateway address into an IPv6 address and make sure
			// pod ip address matches with IPv6 address family.
			setNamespaceExternalGateway(f, "fd00:10:244:2::6")
			podIPs, err := createPod(f.ClientSet, f.Namespace.Name, "test-ipv6-pod")
			e2e.Logf("pod IPs are %v after setting external gw with IPv6 address", podIPs)
			switch podIPFamily {
			case DualStack:
				expectNoError(err)
				o.Expect(getIPFamily(podIPs)).To(o.Equal(DualStack))
			case IPv4:
				// This is an expected failure when pod network in IPv4 address family
				// whereas external gateway is set with IPv6 address
				expectError(err)
			case IPv6:
				expectNoError(err)
				o.Expect(getIPFamily(podIPs)).To(o.Equal(IPv6))
			}
			// Set external gateway address into an IPv4 address and make sure
			// pod ip address matches with IPv4 address family.
			setNamespaceExternalGateway(f, "10.10.10.1")
			podIPs, err = createPod(f.ClientSet, f.Namespace.Name, "test-ipv4-pod")
			e2e.Logf("pod IPs are %v after setting external gw with IPv4 address", podIPs)
			switch podIPFamily {
			case DualStack:
				expectNoError(err)
				o.Expect(getIPFamily(podIPs)).To(o.Equal(DualStack))
			case IPv4:
				expectNoError(err)
				o.Expect(getIPFamily(podIPs)).To(o.Equal(IPv4))
			case IPv6:
				// This is an expected failure when pod network in IPv6 address family
				// whereas external gateway is set with IPv4 address
				expectError(err)
			}
			// Set external gateway address supporting Dual Stack and make sure
			// pod ip address(es) match with desired address family.
			setNamespaceExternalGateway(f, "10.10.10.1,fd00:10:244:2::6")
			podIPs, err = createPod(f.ClientSet, f.Namespace.Name, "test-dual-stack-pod")
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("pod IPs are %v after setting external gw with Dual Stack address", podIPs)
			o.Expect(getIPFamily(podIPs)).To(o.Equal(podIPFamily))
		})
	})
})
