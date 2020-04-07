/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package network

import (
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/master/ports"
	"k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
)

var _ = SIGDescribe("Networking", func() {
	var svcname = "nettest"
	f := framework.NewDefaultFramework(svcname)

	BeforeEach(func() {
		// Assert basic external connectivity.
		// Since this is not really a test of kubernetes in any way, we
		// leave it as a pre-test assertion, rather than a Ginko test.
		By("Executing a successful http request from the external internet")
		resp, err := http.Get("http://google.com")
		if err != nil {
			framework.Failf("Unable to connect/talk to the internet: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			framework.Failf("Unexpected error code, expected 200, got, %v (%v)", resp.StatusCode, resp)
		}
	})

	It("should provide Internet connection for containers [Feature:Networking-IPv4]", func() {
		By("Running container which tries to ping 8.8.8.8")
		framework.ExpectNoError(
			framework.CheckConnectivityToHost(f, "", "ping-test", "8.8.8.8", framework.IPv4PingCommand, 30))
	})

	It("should provide Internet connection for containers [Feature:Networking-IPv6][Experimental]", func() {
		By("Running container which tries to ping google.com")
		framework.ExpectNoError(
			framework.CheckConnectivityToHost(f, "", "ping-test", "google.com", framework.IPv6PingCommand, 30))
	})

	// First test because it has no dependencies on variables created later on.
	It("should provide unchanging, static URL paths for kubernetes api services", func() {
		tests := []struct {
			path string
		}{
			{path: "/healthz"},
			{path: "/api"},
			{path: "/apis"},
			{path: "/metrics"},
			{path: "/openapi/v2"},
			{path: "/version"},
			// TODO: test proxy links here
		}
		if !framework.ProviderIs("gke", "skeleton") {
			tests = append(tests, struct{ path string }{path: "/logs"})
		}
		for _, test := range tests {
			By(fmt.Sprintf("testing: %s", test.path))
			data, err := f.ClientSet.CoreV1().RESTClient().Get().
				AbsPath(test.path).
				DoRaw()
			if err != nil {
				framework.Failf("Failed: %v\nBody: %s", err, string(data))
			}
		}
	})

	It("should check kube-proxy urls", func() {
		// TODO: this is overkill we just need the host networking pod
		// to hit kube-proxy urls.
		config := framework.NewNetworkingTestConfig(f)

		By("checking kube-proxy URLs")
		config.GetSelfURL(ports.ProxyHealthzPort, "/healthz", "200 OK")
		// Verify /healthz returns the proper content.
		config.GetSelfURL(ports.ProxyHealthzPort, "/healthz", "lastUpdated")
		// Verify /proxyMode returns http status code 200.
		config.GetSelfURLStatusCode(ports.ProxyStatusPort, "/proxyMode", "200")
	})

	// TODO: Remove [Slow] when this has had enough bake time to prove presubmit worthiness.
	Describe("Granular Checks: Services [Slow]", func() {

		It("should function for pod-Service: http", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(http) %v --> %v:%v (config.clusterIP)", config.TestContainerPod.Name, config.ClusterIP, framework.ClusterHttpPort))
			config.DialFromTestContainer("http", config.ClusterIP, framework.ClusterHttpPort, config.MaxTries, 0, config.EndpointHostnames())

			By(fmt.Sprintf("dialing(http) %v --> %v:%v (nodeIP)", config.TestContainerPod.Name, config.NodeIP, config.NodeHttpPort))
			config.DialFromTestContainer("http", config.NodeIP, config.NodeHttpPort, config.MaxTries, 0, config.EndpointHostnames())
		})

		It("should function for pod-Service: udp", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(udp) %v --> %v:%v (config.clusterIP)", config.TestContainerPod.Name, config.ClusterIP, framework.ClusterUdpPort))
			config.DialFromTestContainer("udp", config.ClusterIP, framework.ClusterUdpPort, config.MaxTries, 0, config.EndpointHostnames())

			By(fmt.Sprintf("dialing(udp) %v --> %v:%v (nodeIP)", config.TestContainerPod.Name, config.NodeIP, config.NodeUdpPort))
			config.DialFromTestContainer("udp", config.NodeIP, config.NodeUdpPort, config.MaxTries, 0, config.EndpointHostnames())
		})

		It("should function for node-Service: http", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(http) %v (node) --> %v:%v (config.clusterIP)", config.NodeIP, config.ClusterIP, framework.ClusterHttpPort))
			config.DialFromNode("http", config.ClusterIP, framework.ClusterHttpPort, config.MaxTries, 0, config.EndpointHostnames())

			By(fmt.Sprintf("dialing(http) %v (node) --> %v:%v (nodeIP)", config.NodeIP, config.NodeIP, config.NodeHttpPort))
			config.DialFromNode("http", config.NodeIP, config.NodeHttpPort, config.MaxTries, 0, config.EndpointHostnames())
		})

		It("should function for node-Service: udp", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(udp) %v (node) --> %v:%v (config.clusterIP)", config.NodeIP, config.ClusterIP, framework.ClusterUdpPort))
			config.DialFromNode("udp", config.ClusterIP, framework.ClusterUdpPort, config.MaxTries, 0, config.EndpointHostnames())

			By(fmt.Sprintf("dialing(udp) %v (node) --> %v:%v (nodeIP)", config.NodeIP, config.NodeIP, config.NodeUdpPort))
			config.DialFromNode("udp", config.NodeIP, config.NodeUdpPort, config.MaxTries, 0, config.EndpointHostnames())
		})

		It("should function for endpoint-Service: http", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(http) %v (endpoint) --> %v:%v (config.clusterIP)", config.EndpointPods[0].Name, config.ClusterIP, framework.ClusterHttpPort))
			config.DialFromEndpointContainer("http", config.ClusterIP, framework.ClusterHttpPort, config.MaxTries, 0, config.EndpointHostnames())

			By(fmt.Sprintf("dialing(http) %v (endpoint) --> %v:%v (nodeIP)", config.EndpointPods[0].Name, config.NodeIP, config.NodeHttpPort))
			config.DialFromEndpointContainer("http", config.NodeIP, config.NodeHttpPort, config.MaxTries, 0, config.EndpointHostnames())
		})

		It("should function for endpoint-Service: udp", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(udp) %v (endpoint) --> %v:%v (config.clusterIP)", config.EndpointPods[0].Name, config.ClusterIP, framework.ClusterUdpPort))
			config.DialFromEndpointContainer("udp", config.ClusterIP, framework.ClusterUdpPort, config.MaxTries, 0, config.EndpointHostnames())

			By(fmt.Sprintf("dialing(udp) %v (endpoint) --> %v:%v (nodeIP)", config.EndpointPods[0].Name, config.NodeIP, config.NodeUdpPort))
			config.DialFromEndpointContainer("udp", config.NodeIP, config.NodeUdpPort, config.MaxTries, 0, config.EndpointHostnames())
		})

		It("should update endpoints: http", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(http) %v --> %v:%v (config.clusterIP)", config.TestContainerPod.Name, config.ClusterIP, framework.ClusterHttpPort))
			config.DialFromTestContainer("http", config.ClusterIP, framework.ClusterHttpPort, config.MaxTries, 0, config.EndpointHostnames())

			config.DeleteNetProxyPod()

			By(fmt.Sprintf("dialing(http) %v --> %v:%v (config.clusterIP)", config.TestContainerPod.Name, config.ClusterIP, framework.ClusterHttpPort))
			config.DialFromTestContainer("http", config.ClusterIP, framework.ClusterHttpPort, config.MaxTries, config.MaxTries, config.EndpointHostnames())
		})

		It("should update endpoints: udp", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(udp) %v --> %v:%v (config.clusterIP)", config.TestContainerPod.Name, config.ClusterIP, framework.ClusterUdpPort))
			config.DialFromTestContainer("udp", config.ClusterIP, framework.ClusterUdpPort, config.MaxTries, 0, config.EndpointHostnames())

			config.DeleteNetProxyPod()

			By(fmt.Sprintf("dialing(udp) %v --> %v:%v (config.clusterIP)", config.TestContainerPod.Name, config.ClusterIP, framework.ClusterUdpPort))
			config.DialFromTestContainer("udp", config.ClusterIP, framework.ClusterUdpPort, config.MaxTries, config.MaxTries, config.EndpointHostnames())
		})

		// Slow because we confirm that the nodePort doesn't serve traffic, which requires a period of polling.
		It("should update nodePort: http [Slow]", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(http) %v (node) --> %v:%v (nodeIP)", config.NodeIP, config.NodeIP, config.NodeHttpPort))
			config.DialFromNode("http", config.NodeIP, config.NodeHttpPort, config.MaxTries, 0, config.EndpointHostnames())

			config.DeleteNodePortService()

			By(fmt.Sprintf("dialing(http) %v (node) --> %v:%v (nodeIP)", config.NodeIP, config.NodeIP, config.NodeHttpPort))
			config.DialFromNode("http", config.NodeIP, config.NodeHttpPort, config.MaxTries, config.MaxTries, sets.NewString())
		})

		// Slow because we confirm that the nodePort doesn't serve traffic, which requires a period of polling.
		It("should update nodePort: udp [Slow]", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(udp) %v (node) --> %v:%v (nodeIP)", config.NodeIP, config.NodeIP, config.NodeUdpPort))
			config.DialFromNode("udp", config.NodeIP, config.NodeUdpPort, config.MaxTries, 0, config.EndpointHostnames())

			config.DeleteNodePortService()

			By(fmt.Sprintf("dialing(udp) %v (node) --> %v:%v (nodeIP)", config.NodeIP, config.NodeIP, config.NodeUdpPort))
			config.DialFromNode("udp", config.NodeIP, config.NodeUdpPort, config.MaxTries, config.MaxTries, sets.NewString())
		})

		It("should function for client IP based session affinity: http", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(http) %v --> %v:%v", config.TestContainerPod.Name, config.SessionAffinityService.Spec.ClusterIP, framework.ClusterHttpPort))

			// Check if number of endpoints returned are exactly one.
			eps, err := config.GetEndpointsFromTestContainer("http", config.SessionAffinityService.Spec.ClusterIP, framework.ClusterHttpPort, framework.SessionAffinityChecks)
			if err != nil {
				framework.Failf("Failed to get endpoints from test container, error: %v", err)
			}
			if len(eps) == 0 {
				framework.Failf("Unexpected no endpoints return")
			}
			if len(eps) > 1 {
				framework.Failf("Unexpected endpoints return: %v, expect 1 endpoints", eps)
			}
		})

		It("should function for client IP based session affinity: udp", func() {
			config := framework.NewNetworkingTestConfig(f)
			By(fmt.Sprintf("dialing(udp) %v --> %v:%v", config.TestContainerPod.Name, config.SessionAffinityService.Spec.ClusterIP, framework.ClusterUdpPort))

			// Check if number of endpoints returned are exactly one.
			eps, err := config.GetEndpointsFromTestContainer("udp", config.SessionAffinityService.Spec.ClusterIP, framework.ClusterUdpPort, framework.SessionAffinityChecks)
			if err != nil {
				framework.Failf("Failed to get endpoints from test container, error: %v", err)
			}
			if len(eps) == 0 {
				framework.Failf("Unexpected no endpoints return")
			}
			if len(eps) > 1 {
				framework.Failf("Unexpected endpoints return: %v, expect 1 endpoints", eps)
			}
		})

		It("should recreate its iptables rules if they are deleted [Disruptive]", func() {
			framework.SkipUnlessProviderIs(framework.ProvidersWithSSH...)
			framework.SkipUnlessSSHKeyPresent()

			hosts, err := framework.NodeSSHHosts(f.ClientSet)
			framework.ExpectNoError(err, "failed to find external/internal IPs for every node")
			if len(hosts) == 0 {
				framework.Failf("No ssh-able nodes")
			}
			host := hosts[0]

			ns := f.Namespace.Name
			numPods, servicePort := 3, defaultServeHostnameServicePort
			svc := "iptables-flush-test"

			defer func() {
				framework.ExpectNoError(framework.StopServeHostnameService(f.ClientSet, ns, svc))
			}()
			podNames, svcIP, err := framework.StartServeHostnameService(f.ClientSet, f.InternalClientset, getServeHostnameService(svc), ns, numPods)
			framework.ExpectNoError(err, "failed to create replication controller with service: %s in the namespace: %s", svc, ns)

			// Ideally we want to reload the system firewall, but we don't necessarily
			// know how to do that on this system ("firewall-cmd --reload"? "systemctl
			// restart iptables"?). So instead we just manually delete all "KUBE-"
			// chains.

			By("dumping iptables rules on a node")
			result, err := framework.SSH("sudo iptables-save", host, framework.TestContext.Provider)
			if err != nil || result.Code != 0 {
				framework.LogSSHResult(result)
				framework.Failf("couldn't dump iptable rules: %v", err)
			}

			// All the commands that delete rules have to come before all the commands
			// that delete chains, since the chains can't be deleted while there are
			// still rules referencing them.
			var deleteRuleCmds, deleteChainCmds []string
			table := ""
			for _, line := range strings.Split(result.Stdout, "\n") {
				if strings.HasPrefix(line, "*") {
					table = line[1:]
				} else if table == "" {
					continue
				}

				// Delete jumps from non-KUBE chains to KUBE chains
				if !strings.HasPrefix(line, "-A KUBE-") && strings.Contains(line, "-j KUBE-") {
					deleteRuleCmds = append(deleteRuleCmds, fmt.Sprintf("sudo iptables -t %s -D %s || true", table, line[3:]))
				}
				// Flush and delete all KUBE chains
				if strings.HasPrefix(line, ":KUBE-") {
					chain := strings.Split(line, " ")[0][1:]
					deleteRuleCmds = append(deleteRuleCmds, fmt.Sprintf("sudo iptables -t %s -F %s || true", table, chain))
					deleteChainCmds = append(deleteChainCmds, fmt.Sprintf("sudo iptables -t %s -X %s || true", table, chain))
				}
			}
			cmd := strings.Join(append(deleteRuleCmds, deleteChainCmds...), "\n")

			By("deleting all KUBE-* iptables chains")
			result, err = framework.SSH(cmd, host, framework.TestContext.Provider)
			if err != nil || result.Code != 0 {
				framework.LogSSHResult(result)
				framework.Failf("couldn't delete iptable rules: %v", err)
			}

			By("verifying that kube-proxy rules are eventually recreated")
			framework.ExpectNoError(framework.VerifyServeHostnameServiceUp(f.ClientSet, ns, host, podNames, svcIP, servicePort))

			By("verifying that kubelet rules are eventually recreated")
			err = utilwait.PollImmediate(framework.Poll, framework.RestartNodeReadyAgainTimeout, func() (bool, error) {
				result, err = framework.SSH("sudo iptables-save -t nat", host, framework.TestContext.Provider)
				if err != nil || result.Code != 0 {
					framework.LogSSHResult(result)
					return false, err
				}

				if strings.Contains(result.Stdout, "\n-A KUBE-MARK-DROP ") {
					return true, nil
				}
				return false, nil
			})
			framework.ExpectNoError(err, "kubelet did not recreate its iptables rules")
		})
	})
})
