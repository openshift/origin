package etcd

import (
	"context"
	"fmt"
	"net"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

var _ = g.Describe("[sig-etcd] etcd", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("etcd-invariants").AsAdmin()

	g.It("cluster has the same number of master nodes and voting members from the endpoints configmap [Early][apigroup:config.openshift.io]", func() {
		exutil.SkipIfExternalControlplaneTopology(oc, "clusters with external controlplane topology don't have master nodes")
		masterNodeLabelSelectorString := "node-role.kubernetes.io/master"
		controlPlaneNodeList, err := oc.KubeClient().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: masterNodeLabelSelectorString})
		o.Expect(err).ToNot(o.HaveOccurred())

		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).ToNot(o.HaveOccurred())

		if *controlPlaneTopology == configv1.HighlyAvailableArbiterMode {
			arbiterNodeLabelSelectorString := "node-role.kubernetes.io/arbiter"
			arbiterNodeList, err := oc.KubeClient().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: arbiterNodeLabelSelectorString})
			o.Expect(err).ToNot(o.HaveOccurred())
			controlPlaneNodeList.Items = append(controlPlaneNodeList.Items, arbiterNodeList.Items...)
		}

		ipFamily := getCurrentNetworkTopology(oc)
		currentControlPlaneNodesIPListSet := sets.NewString()
		for _, controlPlaneNode := range controlPlaneNodeList.Items {
			for _, nodeAddress := range controlPlaneNode.Status.Addresses {
				if nodeAddress.Type == corev1.NodeInternalIP {
					switch ipFamily {
					case "tcp4":
						isIPv4, err := isIPv4(nodeAddress.Address)
						o.Expect(err).ToNot(o.HaveOccurred())
						if isIPv4 {
							currentControlPlaneNodesIPListSet.Insert(nodeAddress.Address)
						}
					case "tcp6":
						isIPv4, err := isIPv4(nodeAddress.Address)
						o.Expect(err).ToNot(o.HaveOccurred())
						if !isIPv4 {
							currentControlPlaneNodesIPListSet.Insert(nodeAddress.Address)
						}
					default:
						g.GinkgoT().Fatalf("unexpected ip family: %q", ipFamily)
					}
				}
			}
		}
		etcdEndpointsConfigMap, err := oc.KubeClient().CoreV1().ConfigMaps("openshift-etcd").Get(context.TODO(), "etcd-endpoints", metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		currentVotingMemberIPListSet := sets.NewString()
		for _, votingMemberIP := range etcdEndpointsConfigMap.Data {
			currentVotingMemberIPListSet.Insert(votingMemberIP)
		}

		if currentVotingMemberIPListSet.Len() != currentControlPlaneNodesIPListSet.Len() {
			g.GinkgoT().Fatalf(
				"incorrect number of voting members found in openshift-etcd/etcd-endpoints, expected it to match the number of master nodes = %d, members from the cm = %v, master nodes = %v",
				currentControlPlaneNodesIPListSet.Len(),
				currentVotingMemberIPListSet.List(),
				currentControlPlaneNodesIPListSet.List())
		}

		if !currentVotingMemberIPListSet.Equal(currentControlPlaneNodesIPListSet) {
			g.GinkgoT().Fatalf("IPs of voting members from openshift-etcd/etcd-endpoints =%v don't match master nodes IPs = %v ", currentVotingMemberIPListSet.List(), currentControlPlaneNodesIPListSet.List())
		}
	})
})

func getCurrentNetworkTopology(oc *exutil.CLI) string {
	configClient, err := configv1client.NewForConfig(oc.AdminConfig())
	o.Expect(err).ToNot(o.HaveOccurred())
	nework, err := configClient.ConfigV1().Networks().Get(context.TODO(), "cluster", metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
	ipFamily, err := getPreferredIPFamily(nework)
	o.Expect(err).ToNot(o.HaveOccurred())
	return ipFamily
}

// GetPreferredIPFamily checks network status for service CIDR to conclude IP family. If status is not yet populated fallback to spec.
func getPreferredIPFamily(network *configv1.Network) (string, error) {
	var serviceCIDR string
	switch {
	case len(network.Status.ServiceNetwork) != 0:
		serviceCIDR = network.Status.ServiceNetwork[0]
		if len(serviceCIDR) == 0 {
			return "", fmt.Errorf("networks.%s/cluster: status.serviceNetwork[0] is empty", configv1.GroupName)
		}
		break
	case len(network.Spec.ServiceNetwork) != 0:
		klog.Warningf("networks.%s/cluster: status.serviceNetwork not found falling back to spec.serviceNetwork", configv1.GroupName)
		serviceCIDR = network.Spec.ServiceNetwork[0]
		if len(serviceCIDR) == 0 {
			return "", fmt.Errorf("networks.%s/cluster: spec.serviceNetwork[0] is empty", configv1.GroupName)
		}
		break
	default:
		return "", fmt.Errorf("networks.%s/cluster: status|spec.serviceNetwork not found", configv1.GroupName)
	}

	ip, _, err := net.ParseCIDR(serviceCIDR)

	switch {
	case err != nil:
		return "", err
	case ip.To4() == nil:
		return "tcp6", nil
	default:
		return "tcp4", nil
	}
}

func isIPv4(ipString string) (bool, error) {
	ip := net.ParseIP(ipString)

	switch {
	case ip == nil:
		return false, fmt.Errorf("not an IP")
	case ip.To4() == nil:
		return false, nil
	default:
		return true, nil
	}
}
