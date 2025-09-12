package two_node

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ensurePodmanEtcdContainerIsRunning = "podman inspect --format '{{.State.Running}}' etcd"

var _ = g.Describe("[sig-node][apigroup:config.openshift.io][OCPFeatureGate:DualReplica] Two Node with Fencing topology", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("")
	)

	g.BeforeEach(func() {
		skipIfNotTopology(oc, v1.DualReplicaTopologyMode)
	})

	g.It("should only have two control plane nodes and no arbiter nodes", func() {
		const (
			expectedControlPlanes = 2
			expectedArbiters      = 0
		)

		g.By(fmt.Sprintf("Ensuring only %d control-plane nodes in the cluster and no arbiter nodes", expectedControlPlanes))
		controlPlaneNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: labelNodeRoleControlPlane,
		})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve control-plane nodes without error")
		o.Expect(len(controlPlaneNodes.Items)).To(o.Equal(expectedControlPlanes), fmt.Sprintf("Expected %d Control-plane Nodes, found %d", expectedControlPlanes, len(controlPlaneNodes.Items)))

		arbiterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: labelNodeRoleArbiter,
		})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve arbiter nodes without error")
		o.Expect(len(arbiterNodes.Items)).To(o.Equal(expectedArbiters), fmt.Sprintf("Expected %d Arbiter Nodes, found %d", expectedArbiters, len(arbiterNodes.Items)))
	})
})

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica] Two Node with Fencing", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("")
	)

	g.BeforeEach(func() {
		skipIfNotTopology(oc, v1.DualReplicaTopologyMode)
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
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: labelNodeRoleControlPlane,
		})
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
