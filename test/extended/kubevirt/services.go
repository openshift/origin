package kubevirt

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = Describe("[sig-kubevirt] services", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("ns-global", admissionapi.LevelBaseline)

	InKubeVirtClusterContext(oc, func() {
		mgmtFramework := e2e.NewDefaultFramework("mgmt-framework")
		mgmtFramework.SkipNamespaceCreation = true

		f1 := e2e.NewDefaultFramework("server-framework")
		f1.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

		It("should allow connections to pods from infra cluster pod via NodePort across different infra nodes", Label("Size:M"), func() {
			oc = SetMgmtFramework(mgmtFramework)
			// This tests connectivity from the infra cluster's pod network to a NodePort
			// within a nested KubeVirt Hypershift guest cluster.
			//
			// This exercises the back half of the network flow used to pass ingress
			// through the ingress cluster into the nested guest cluster.
			//
			// client pod (on infra cluster) -> guest node IP -> NodePort -> server pod (on guest cluster)
			//
			Expect(checkKubeVirtInfraClusterNodePortConnectivity(f1, mgmtFramework, oc)).To(Succeed())
		})

		It("should allow connections to pods from guest hostNetwork pod via NodePort across different guest nodes", Label("Size:M"), func() {
			// Within a nested KubeVirt guest cluster, this tests the ability for
			// NodePort services to route from hostnet to pods across guest nodes.
			Expect(checkKubeVirtGuestClusterHostNetworkNodePortConnectivity(f1, f1)).To(Succeed())
		})

		It("should allow connections to pods from guest podNetwork pod via NodePort across different guest nodes", Label("Size:M"), func() {
			// Within a nested KubeVirt guest cluster, this tests the ability for
			// NodePort services to route from pod network to pods across guest nodes.
			Expect(checkKubeVirtGuestClusterPodNetworkNodePortConnectivity(f1, f1)).To(Succeed())
		})

		It("should allow direct connections to pods from guest cluster pod in pod network across different guest nodes", Label("Size:M"), func() {
			// Within a nested KubeVirt guest cluster, this tests the ability for different pods within the
			// guest cluster to communicate each other, across different guest cluster nodes, via PodNetwork.
			Expect(checkKubeVirtGuestClusterPodNetworkConnectivity(f1, f1)).To(Succeed())
		})

		It("should allow direct connections to pods from guest cluster pod in host network across different guest nodes", Label("Size:M"), func() {
			// Within a nested KubeVirt guest cluster, this tests the ability for different pods within the
			// guest cluster to communicate each other, across different guest cluster nodes, via HostNetwork.
			Expect(checkKubeVirtGuestClusterHostNetworkConnectivity(f1, f1)).To(Succeed())
		})

		It("should allow connections to pods from infra cluster pod via LoadBalancer service across different guest nodes", Label("Size:L"), func() {
			oc = SetMgmtFramework(mgmtFramework)
			// Within a nested KubeVirt guest cluster, this tests the ability for
			// LoadBalancer services to route from pod network to pods across infra nodes.
			// client pod (on infra cluster) -> guest node IP -> LoadBalancer Service -> server pod (on guest cluster)
			Expect(checkKubeVirtInfraClusterLoadBalancerConnectivity(f1, mgmtFramework, oc)).To(Succeed())
		})

		It("should allow connections to pods from guest cluster PodNetwork pod via LoadBalancer service across different guest nodes", Label("Size:L"), func() {
			// Within a nested KubeVirt guest cluster, this tests the ability for
			// LoadBalancer services to route from pod network to pods across guest nodes.
			Expect(checkKubeVirtGuestClusterLoadBalancerConnectivity(f1, f1)).To(Succeed())
		})
	})

})
