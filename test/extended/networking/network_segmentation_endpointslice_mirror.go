package networking

import (
	"context"
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	utilnet "k8s.io/utils/net"
	"net"
)

// parseHostSubnet parses the "k8s.ovn.org/host-cidrs" annotation and returns the v4 and v6 host CIDRs
func parseHostSubnet(node *corev1.Node) (string, string, error) {
	const ovnNodeHostCIDRAnnot = "k8s.ovn.org/host-cidrs"
	var v4HostSubnet, v6HostSubnet string
	addrAnnotation, ok := node.Annotations[ovnNodeHostCIDRAnnot]
	if !ok {
		return "", "", fmt.Errorf("%s annotation not found for node %q", ovnNodeHostCIDRAnnot, node.Name)
	}

	var subnets []string
	if err := json.Unmarshal([]byte(addrAnnotation), &subnets); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal %s annotation %s for node %q: %v", ovnNodeHostCIDRAnnot,
			addrAnnotation, node.Name, err)
	}

	for _, subnet := range subnets {
		ipFamily := utilnet.IPFamilyOfCIDRString(subnet)
		if ipFamily == utilnet.IPv6 {
			v6HostSubnet = subnet
		} else if ipFamily == utilnet.IPv4 {
			v4HostSubnet = subnet
		}
	}
	return v4HostSubnet, v6HostSubnet, nil
}

func validateMirroredEndpointSlices(cs clientset.Interface, namespace, svcName, expectedV4Subnet, expectedV6Subnet string, expectedEndpoints int, isDualStack, isHostNetwork bool) error {
	esList, err := cs.DiscoveryV1().EndpointSlices(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", "k8s.ovn.org/service-name", svcName)})
	if err != nil {
		return err
	}

	expectedEndpointSlicesCount := 1
	if isDualStack {
		expectedEndpointSlicesCount = 2
	}
	if len(esList.Items) != expectedEndpointSlicesCount {
		return fmt.Errorf("expected %d mirrored EndpointSlice, got: %d", expectedEndpointSlicesCount, len(esList.Items))
	}

	for _, endpointSlice := range esList.Items {
		if len(endpointSlice.Endpoints) != expectedEndpoints {
			return fmt.Errorf("expected %d endpoints, got: %d", expectedEndpoints, len(esList.Items))
		}

		subnet := expectedV4Subnet
		if endpointSlice.AddressType == discoveryv1.AddressTypeIPv6 {
			subnet = expectedV6Subnet
		}

		for _, endpoint := range endpointSlice.Endpoints {
			if len(endpoint.Addresses) != 1 {
				return fmt.Errorf("expected 1 endpoint, got: %d", len(endpoint.Addresses))
			}

			if isHostNetwork {
				if endpoint.NodeName == nil {
					return fmt.Errorf("expected node name for endpoint, got: nil")
				}

				nodeIP, err := getNodeIP(cs, *endpoint.NodeName, endpointSlice.AddressType == discoveryv1.AddressTypeIPv6)
				if err != nil {
					return err
				}
				if !nodeIP.Equal(net.ParseIP(endpoint.Addresses[0])) {
					return fmt.Errorf("ip %q is not equal to the node IP %v", endpoint.Addresses[0], nodeIP)
				}
			} else {
				if err := inRange(subnet, endpoint.Addresses[0]); err != nil {
					return err
				}
			}

		}
	}
	return nil
}

func getNodeIP(cs clientset.Interface, nodeName string, isIPv6 bool) (net.IP, error) {
	node, err := cs.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP && utilnet.IsIPv6String(addr.Address) == isIPv6 {
			return net.ParseIP(addr.Address), nil
		}
	}
	return nil, fmt.Errorf("no matching node IPs found in %s", node.Status.Addresses)
}
