package networking

import (
	"context"
	"fmt"
	"regexp"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-network] external gateway address", func() {
	oc := exutil.NewCLIWithoutNamespace("ns-global")

	InOVNKubernetesContext(func() {
		f := oc.KubeFramework()
		f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

		g.It("should match the address family of the pod", func() {
			labelKey, labelValue := "test", "external-gateway"
			labels := map[string]string{
				labelKey: labelValue,
			}

			// Expected error message for APB policy sync failure
			errorLog := "Failed to sync APB policy %s.*gateway specified for namespace %s.*%s"

			// Returns true if the ovnkube-controller logs contain the expected error message
			checkLogs := func(ovnkubePodInfo ovnKubePodInfo, regex *regexp.Regexp) (bool, error) {
				logs, err := e2epod.GetPodLogs(context.TODO(), f.ClientSet, ovnNamespace, ovnkubePodInfo.podName, ovnkubePodInfo.containerName)
				if err != nil {
					return false, err
				}
				return regex.MatchString(logs), nil
			}

			g.By("creating namespace with external-gateway label")
			ns, err := f.CreateNamespace(context.TODO(), f.BaseName, labels)
			expectNoError(err)
			f.Namespace = ns

			g.By("determining cluster pod IP address family")
			podIPFamily := GetIPFamilyForCluster(f)
			o.Expect(podIPFamily).NotTo(o.Equal(Unknown))

			// Set external gateway address into an IPv6 address and make sure
			// pod ip address matches with IPv6 address family.
			apbPolicyNameIPv6 := "static-egress-route-ipv6"
			g.By(fmt.Sprintf("applying IPv6 AdminPolicyBasedExternalRoute %s with gateway fd00:10:244:2::6", apbPolicyNameIPv6))
			setNamespaceExternalGateway(apbPolicyNameIPv6, []string{"fd00:10:244:2::6"}, labelKey, labelValue)
			g.DeferCleanup(func() {
				g.By(fmt.Sprintf("deleting AdminPolicyBasedExternalRoute %s", apbPolicyNameIPv6))
				oc.AsAdmin().WithoutNamespace().Run("delete").Args("adminpolicybasedexternalroute", apbPolicyNameIPv6, "--ignore-not-found").Execute()
			})

			podNameIPv6 := "test-ipv6-pod"
			g.By(fmt.Sprintf("creating pod %s in namespace %s", podNameIPv6, f.Namespace.Name))
			pod, err := createPod(f.ClientSet, f.Namespace.Name, podNameIPv6)
			expectNoError(err)
			g.DeferCleanup(func() {
				g.By(fmt.Sprintf("deleting pod %s", podNameIPv6))
				oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", podNameIPv6, "-n", f.Namespace.Name, "--ignore-not-found").Execute()
			})
			podIPs := pod.Status.PodIPs
			e2e.Logf("pod IPs are %v after setting external gw with IPv6 address", podIPs)

			g.By(fmt.Sprintf("finding ovnkube-node pod on node %s", pod.Spec.NodeName))
			ovnkubePodInfo, err := ovnkubePod(oc, pod.Spec.NodeName)
			expectNoError(err)

			regexIPv6, err := regexp.Compile(fmt.Sprintf(errorLog, apbPolicyNameIPv6, f.Namespace.Name, podNameIPv6))
			expectNoError(err)
			switch podIPFamily {
			case DualStack, IPv6:
				g.By(fmt.Sprintf("verifying ovnkube-node logs do not report APB sync failure for IPv6 gateway on %s cluster", podIPFamily))
				o.Consistently(func() bool {
					found, err := checkLogs(ovnkubePodInfo, regexIPv6)
					if err != nil {
						e2e.Logf("Error checking logs: %v", err)
						return true
					}
					return found
				}).
					WithPolling(20 * time.Second).
					WithTimeout(1 * time.Minute).
					Should(o.BeFalse())
			case IPv4:
				// This is an expected failure when pod network in IPv4 address family
				// whereas external gateway is set with IPv6 address
				g.By("verifying ovnkube-node logs report APB sync failure for mismatched IPv6 gateway on IPv4 cluster")
				o.Eventually(func() bool {
					found, err := checkLogs(ovnkubePodInfo, regexIPv6)
					if err != nil {
						e2e.Logf("Error checking logs: %v", err)
						return false
					}
					return found
				}).
					WithPolling(20 * time.Second).
					WithTimeout(1 * time.Minute).
					Should(o.BeTrue())
			}

			// Set external gateway address into an IPv4 address and make sure
			// pod ip address matches with IPv4 address family.
			apbPolicyNameIPv4 := "static-egress-route-ipv4"
			g.By(fmt.Sprintf("applying IPv4 AdminPolicyBasedExternalRoute %s with gateway 10.10.10.1", apbPolicyNameIPv4))
			setNamespaceExternalGateway(apbPolicyNameIPv4, []string{"10.10.10.1"}, labelKey, labelValue)
			g.DeferCleanup(func() {
				g.By(fmt.Sprintf("deleting AdminPolicyBasedExternalRoute %s", apbPolicyNameIPv4))
				oc.AsAdmin().WithoutNamespace().Run("delete").Args("adminpolicybasedexternalroute", apbPolicyNameIPv4, "--ignore-not-found").Execute()
			})

			podNameIPv4 := "test-ipv4-pod"
			g.By(fmt.Sprintf("creating pod %s in namespace %s", podNameIPv4, f.Namespace.Name))
			pod, err = createPod(f.ClientSet, f.Namespace.Name, podNameIPv4)
			expectNoError(err)
			g.DeferCleanup(func() {
				g.By(fmt.Sprintf("deleting pod %s", podNameIPv4))
				oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", podNameIPv4, "-n", f.Namespace.Name, "--ignore-not-found").Execute()
			})
			podIPs = pod.Status.PodIPs
			e2e.Logf("pod IPs are %v after setting external gw with IPv4 address", podIPs)

			g.By(fmt.Sprintf("finding ovnkube-node pod on node %s", pod.Spec.NodeName))
			ovnkubePodInfo, err = ovnkubePod(oc, pod.Spec.NodeName)
			expectNoError(err)

			regexIPv4, err := regexp.Compile(fmt.Sprintf(errorLog, apbPolicyNameIPv4, f.Namespace.Name, podNameIPv4))
			expectNoError(err)
			switch podIPFamily {
			case DualStack, IPv4:
				g.By(fmt.Sprintf("verifying ovnkube-node logs do not report APB sync failure for IPv4 gateway on %s cluster", podIPFamily))
				o.Consistently(func() bool {
					found, err := checkLogs(ovnkubePodInfo, regexIPv4)
					if err != nil {
						e2e.Logf("Error checking logs: %v", err)
						return true
					}
					return found
				}).
					WithPolling(20 * time.Second).
					WithTimeout(1 * time.Minute).
					Should(o.BeFalse())
			case IPv6:
				// This is an expected failure when pod network in IPv6 address family
				// whereas external gateway is set with IPv4 address
				g.By("verifying ovnkube-node logs report APB sync failure for mismatched IPv4 gateway on IPv6 cluster")
				o.Eventually(func() bool {
					found, err := checkLogs(ovnkubePodInfo, regexIPv4)
					if err != nil {
						e2e.Logf("Error checking logs: %v", err)
						return false
					}
					return found
				}).
					WithPolling(20 * time.Second).
					WithTimeout(1 * time.Minute).
					Should(o.BeTrue())
			}

			// Set external gateway address supporting Dual Stack and make sure
			// pod ip address(es) match with desired address family.
			apbPolicyNameDualStack := "static-egress-route-dual-stack"
			g.By(fmt.Sprintf("applying dual-stack AdminPolicyBasedExternalRoute %s with gateways 10.10.10.1 and fd00:10:244:2::6", apbPolicyNameDualStack))
			setNamespaceExternalGateway(apbPolicyNameDualStack, []string{"10.10.10.1", "fd00:10:244:2::6"}, labelKey, labelValue)
			g.DeferCleanup(func() {
				g.By(fmt.Sprintf("deleting AdminPolicyBasedExternalRoute %s", apbPolicyNameDualStack))
				oc.AsAdmin().WithoutNamespace().Run("delete").Args("adminpolicybasedexternalroute", apbPolicyNameDualStack, "--ignore-not-found").Execute()
			})

			podNameDualStack := "test-dual-stack-pod"
			g.By(fmt.Sprintf("creating pod %s in namespace %s", podNameDualStack, f.Namespace.Name))
			pod, err = createPod(f.ClientSet, f.Namespace.Name, podNameDualStack)
			expectNoError(err)
			g.DeferCleanup(func() {
				g.By(fmt.Sprintf("deleting pod %s", podNameDualStack))
				oc.AsAdmin().WithoutNamespace().Run("delete").Args("pod", podNameDualStack, "-n", f.Namespace.Name, "--ignore-not-found").Execute()
			})
			podIPs = pod.Status.PodIPs
			e2e.Logf("pod IPs are %v after setting external gw with Dual Stack address", podIPs)

			g.By("verifying pod IP address family matches cluster")
			o.Expect(getIPFamily(podIPs)).To(o.Equal(podIPFamily))

			g.By(fmt.Sprintf("finding ovnkube-node pod on node %s", pod.Spec.NodeName))
			ovnkubePodInfo, err = ovnkubePod(oc, pod.Spec.NodeName)
			expectNoError(err)

			regexDualStack, err := regexp.Compile(fmt.Sprintf(errorLog, apbPolicyNameDualStack, f.Namespace.Name, podNameDualStack))
			expectNoError(err)
			g.By("verifying ovnkube-node logs do not report APB sync failure for dual-stack gateway")
			o.Consistently(func() bool {
				found, err := checkLogs(ovnkubePodInfo, regexDualStack)
				if err != nil {
					e2e.Logf("Error checking logs: %v", err)
					return true
				}
				return found
			}).
				WithPolling(20 * time.Second).
				WithTimeout(1 * time.Minute).
				Should(o.BeFalse())
		})
	})
})
