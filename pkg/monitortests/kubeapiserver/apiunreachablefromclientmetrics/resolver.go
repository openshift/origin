package apiunreachablefromclientmetrics

import (
	"context"
	"fmt"
	"net"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// TODO: move this to a reusable package
func NewClusterInfoResolver(ctx context.Context, client kubernetes.Interface) (*clusterInfoResolver, error) {
	kubeSvc, err := client.CoreV1().Services(metav1.NamespaceDefault).Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve cluster IP from kubernetes.default.svc - %v", err)
	}

	allNodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve nodes - %v", err)
	}
	if len(allNodes.Items) == 0 {
		return nil, fmt.Errorf("unexpected empty list of nodes")
	}
	return &clusterInfoResolver{
		serviceNetworkIP: kubeSvc.Spec.ClusterIP,
		cache:            populate(allNodes.Items),
	}, nil
}

type nodeNameRole struct {
	name, role string
}

type clusterInfoResolver struct {
	serviceNetworkIP string
	// pre computed hash of IP address to node name and role
	cache map[string]nodeNameRole
}

func (r *clusterInfoResolver) GetKubernetesServiceClusterIP() string { return r.serviceNetworkIP }
func (r *clusterInfoResolver) GetNodeNameAndRoleFromInstance(instance string) (string, string, error) {
	if len(instance) == 0 {
		return "", "", fmt.Errorf("instance name is empty")
	}
	instanceIP := instance
	if strings.Contains(instance, ":") {
		host, _, err := net.SplitHostPort(instance)
		if err != nil {
			return "", "", fmt.Errorf("failed to get node from instance: %s - %w", instance, err)
		}
		instanceIP = host
	}

	match, ok := r.cache[instanceIP]
	if !ok {
		return "", "", fmt.Errorf("did not find a matching node for: %s", instance)
	}
	return match.name, match.role, nil
}

func populate(nodes []corev1.Node) map[string]nodeNameRole {
	cache := map[string]nodeNameRole{}
	for i := range nodes {
		for _, address := range nodes[i].Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				role := getNodeRole(&nodes[i])
				// TODO: should we get the host name from the addresses in the status?
				cache[address.Address] = nodeNameRole{name: nodes[i].Name, role: role}
			}
		}
	}
	return cache
}

func getNodeRole(node *corev1.Node) string {
	if _, ok := node.Labels["node-role.kubernetes.io/worker"]; ok {
		return "worker"
	}
	if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
		return "master"
	}
	return ""
}
