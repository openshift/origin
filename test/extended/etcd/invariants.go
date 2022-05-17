package etcd

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var _ = g.Describe("[sig-etcd] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-invariants").AsAdmin()

	g.It("cluster has the same number of master nodes and voting members from the endpoints configmap [Early]", func() {
		exutil.SkipIfExternalControlplaneTopology(oc, "clusters with external controlplane topology don't have master nodes")
		masterNodeLabelSelectorString := "node-role.kubernetes.io/master"
		masterNodeList, err := oc.KubeClient().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: masterNodeLabelSelectorString})
		o.Expect(err).ToNot(o.HaveOccurred())
		currentMasterNodesIPListSet := sets.NewString()
		for _, masterNode := range masterNodeList.Items {
			for _, masterNodeAddress := range masterNode.Status.Addresses {
				if masterNodeAddress.Type == corev1.NodeInternalIP {
					currentMasterNodesIPListSet.Insert(masterNodeAddress.Address)
				}
			}
		}

		etcdEndpointsConfigMap, err := oc.KubeClient().CoreV1().ConfigMaps("openshift-etcd").Get(context.TODO(), "etcd-endpoints", metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		currentVotingMemberIPListSet := sets.NewString()
		for _, votingMemberIP := range etcdEndpointsConfigMap.Data {
			currentVotingMemberIPListSet.Insert(votingMemberIP)
		}

		if currentVotingMemberIPListSet.Len() != currentMasterNodesIPListSet.Len() {
			g.GinkgoT().Fatalf(
				"incorrect number of voting members found in openshift-etcd/etcd-endpoints, expected it to match the number of master nodes = %d, members from the cm = %v, master nodes = %v",
				currentMasterNodesIPListSet.Len(),
				currentVotingMemberIPListSet.List(),
				currentMasterNodesIPListSet.List())
		}

		if !currentVotingMemberIPListSet.Equal(currentMasterNodesIPListSet) {
			g.GinkgoT().Fatalf("IPs of voting members from openshift-etcd/etcd-endpoints =%v don't match master nodes IPs = %v ", currentVotingMemberIPListSet.List(), currentMasterNodesIPListSet.List())
		}
	})
})
