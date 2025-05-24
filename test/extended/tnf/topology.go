package tnf

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const useHelpers = true

const (
	labelNodeRoleMaster       = "node-role.kubernetes.io/master"
	labelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	labelNodeRoleWorker       = "node-role.kubernetes.io/worker"
	labelNodeRoleArbiter      = "node-role.kubernetes.io/arbiter"
)

const ensurePodmanEtcdContainerIsRunning = "podman inspect --format '{{.State.Running}}' etcd | grep true"

var _ = g.Describe("[sig-node][apigroup:config.openshift.io] Two Nodes OCP with fencing topology", func() {
	defer g.GinkgoRecover()
	var (
		oc    = exutil.NewCLIWithoutNamespace("")
		nodes *corev1.NodeList
	)

	g.BeforeEach(func() {
		infraStatus := getInfraStatus(oc)
		if infraStatus.ControlPlaneTopology != v1.DualReplicaTopologyMode {
			g.Skip("Cluster is not in DualReplicaTopologyMode skipping test")
		}
		var err error
		nodes, err = oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve all nodes without error")
	})

	g.It("Should validate the number of control-planes, workers and arbiters as configured", func() {
		const (
			expectedControlPlanes = 2
			expectedWorkers       = 2 // CPs will also have the Workers label
			expectedArbiters      = 0
		)

		controlPlaneNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: labelNodeRoleControlPlane,
		})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve control-plane nodes without error")
		o.Expect(len(controlPlaneNodes.Items)).To(o.Equal(expectedControlPlanes), fmt.Sprintf("Expected %d Control-plane Nodes, found %d", expectedControlPlanes, len(controlPlaneNodes.Items)))

		workerNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: labelNodeRoleWorker,
		})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve worker nodes without error")
		o.Expect(len(workerNodes.Items)).To(o.Equal(expectedWorkers), fmt.Sprintf("Expected %d Worker Nodes, found %d", expectedWorkers, len(workerNodes.Items)))

		arbiterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: labelNodeRoleArbiter,
		})
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve arbiter nodes without error")
		o.Expect(len(arbiterNodes.Items)).To(o.Equal(expectedArbiters), fmt.Sprintf("Expected %d Arbiter Nodes, found %d", expectedArbiters, len(arbiterNodes.Items)))
	})

	g.It("Should validate the number of etcd pods and containers as configured", func() {
		const (
			expectedEtcdPod           = 2
			expectedEtcdCtlContainers = 2
			expectedEtcdContainers    = 0
		)

		nodeNameA := nodes.Items[0].Name
		nodeNameB := nodes.Items[1].Name

		g.By("Ensuring 0 etcd pod containers and 2 etcdctl pod containers are running in the cluster ")
		pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(context.Background(), metav1.ListOptions{})
		o.Expect(err).To(o.BeNil(), "Expected to retrieve etcd pods in openshift-etcd namespace without error")

		etcdPodCount := 0
		etcdContainerCount := 0
		etcdctlContainerCount := 0
		for _, pod := range pods.Items {
			if pod.Name == "etcd-"+nodeNameA || pod.Name == "etcd-"+nodeNameB {
				etcdPodCount += 1
				for _, container := range pod.Spec.Containers {
					if container.Name == "etcd" {
						etcdContainerCount += 1
					}
					if container.Name == "etcdctl" {
						etcdctlContainerCount += 1
					}
				}
			}
		}
		o.Expect(etcdPodCount).To(o.Equal(expectedEtcdPod))
		o.Expect(etcdctlContainerCount).To(o.Equal(expectedEtcdCtlContainers))
		o.Expect(etcdContainerCount).To(o.Equal(expectedEtcdContainers))
	})

	g.It("Should verify the number of podman-etcd containers as configured", func() {
		g.By("Ensuring one podman etcd container is running on each node")
		for _, node := range nodes.Items {
			_, _, err := runOnNodeNS(oc, node.Name, "openshift-etcd", ensurePodmanEtcdContainerIsRunning)
			o.Expect(err).To(o.BeNil(), fmt.Sprintf("expected a podman etcd container running on Node %s", node.Name))
		}
	})
})
