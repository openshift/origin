package two_node

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/two_node/utils"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const ensurePodmanEtcdContainerIsRunning = "podman inspect --format '{{.State.Running}}' etcd"

var _ = g.Describe("[sig-node][apigroup:config.openshift.io][OCPFeatureGate:DualReplica] Two Node with Fencing topology", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("")
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
	})

	g.It("should only have two control plane nodes and no arbiter nodes", func() {
		const (
			expectedControlPlanes = 2
			expectedArbiters      = 0
		)

		g.By(fmt.Sprintf("Ensuring only %d control-plane nodes in the cluster and no arbiter nodes", expectedControlPlanes))
		controlPlaneNodes, err := utils.GetNodes(oc, utils.LabelNodeRoleControlPlane)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve control-plane nodes without error")
		o.Expect(len(controlPlaneNodes.Items)).To(o.Equal(expectedControlPlanes), fmt.Sprintf("Expected %d Control-plane Nodes, found %d", expectedControlPlanes, len(controlPlaneNodes.Items)))

		arbiterNodes, err := utils.GetNodes(oc, utils.LabelNodeRoleArbiter)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve arbiter nodes without error")
		o.Expect(len(arbiterNodes.Items)).To(o.Equal(expectedArbiters), fmt.Sprintf("Expected %d Arbiter Nodes, found %d", expectedArbiters, len(arbiterNodes.Items)))
	})

	g.It("should have infrastructure platform type set correctly", func() {
		g.By("Checking that the infrastructure platform is set to baremetal or none or external")
		infrastructure, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve infrastructure configuration without error")

		platformType := infrastructure.Status.PlatformStatus.Type
		o.Expect(platformType).To(o.Or(o.Equal(v1.BareMetalPlatformType), o.Equal(v1.NonePlatformType), o.Equal(v1.ExternalPlatformType)),
			fmt.Sprintf("Expected infrastructure platform to be baremetal or none or external, but found %s", platformType))
	})

	g.It("should have BareMetalHost operational status set to detached if they exist", func() {
		g.By("Checking that BareMetalHost objects have operational status set to detached")
		dc := oc.AdminDynamicClient()

		// Use Dynamic Client to get BareMetalHost objects
		// Note: move this to common.go if this is used in other tests
		baremetalGVR := schema.GroupVersionResource{Group: "metal3.io", Resource: "baremetalhosts", Version: "v1alpha1"}
		baremetalClient := dc.Resource(baremetalGVR).Namespace("openshift-machine-api")

		hosts, err := baremetalClient.List(context.Background(), metav1.ListOptions{})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve BareMetalHost objects without error")

		// If no BareMetalHost objects exist, skip the test, this is valid for TNF deployments
		if len(hosts.Items) == 0 {
			g.Skip("No BareMetalHost objects found in openshift-machine-api namespace")
		}

		// Check each BareMetalHost has operational status set to detached
		for _, host := range hosts.Items {
			operationalStatus, found, err := unstructured.NestedString(host.Object, "status", "operationalStatus")
			o.Expect(err).ShouldNot(o.HaveOccurred(), fmt.Sprintf("Expected to parse operational status for BareMetalHost %s without error", host.GetName()))
			o.Expect(found).To(o.BeTrue(), fmt.Sprintf("Expected operational status field to exist for BareMetalHost %s", host.GetName()))
			o.Expect(operationalStatus).To(o.Equal("detached"),
				fmt.Sprintf("Expected BareMetalHost %s operational status to be 'detached', but found '%s'", host.GetName(), operationalStatus))
		}
	})

})

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica] Two Node with Fencing", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("")
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
	})
	g.It("should have etcd pods and containers configured correctly", func() {
		const (
			expectedEtcdPod           = 2
			expectedEtcdCtlContainers = 2
			expectedEtcdContainers    = 0
		)

		g.By("Ensuring 0 etcd pod containers and 2 etcdctl pod containers are running in the cluster ")
		pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=etcd",
		})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve etcd pods in openshift-etcd namespace without error")
		o.Expect(pods.Items).To(o.HaveLen(expectedEtcdPod), "Expected to retrieve %d etcd pods in openshift-etcd namespace", expectedEtcdPod)

		etcdContainerCount := 0
		etcdctlContainerCount := 0
		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "etcd" {
					etcdContainerCount += 1
				}
				if container.Name == "etcdctl" {
					etcdctlContainerCount += 1
				}
			}
		}
		o.Expect(etcdctlContainerCount).To(o.Equal(expectedEtcdCtlContainers))
		o.Expect(etcdContainerCount).To(o.Equal(expectedEtcdContainers))
	})

	g.It("should have podman etcd containers running on each node", func() {
		nodes, err := utils.GetNodes(oc, utils.LabelNodeRoleControlPlane)
		o.Expect(err).To(o.BeNil(), "Expected to retrieve control plane nodes without error")
		o.Expect(nodes.Items).To(o.HaveLen(2), "Expected to retrieve two control plane nodes for DualReplica topology")

		g.By("Ensuring one podman etcd container is running on each node")
		for _, node := range nodes.Items {
			got, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, node.Name, "openshift-etcd", strings.Split(ensurePodmanEtcdContainerIsRunning, " ")...)
			o.Expect(err).To(o.BeNil(), fmt.Sprintf("expected to call podman without errors on Node %s: error %v", node.Name, err))
			o.Expect(got).To(o.Equal("'true'"), fmt.Sprintf("expected a podman etcd container running on Node %s: got running %s", node.Name, got))
		}
	})
})
