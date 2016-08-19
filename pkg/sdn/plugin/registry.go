package plugin

import (
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	osapi "github.com/openshift/origin/pkg/sdn/api"
)

type NetworkInfo struct {
	ClusterNetwork   *net.IPNet
	ServiceNetwork   *net.IPNet
	HostSubnetLength uint32
	PluginName       string
}

type Registry struct {
	oClient *osclient.Client
	kClient *kclient.Client

	// Cache cluster network information
	NetworkInfo *NetworkInfo
}

type ResourceName string

const (
	Nodes                 ResourceName = "Nodes"
	Namespaces            ResourceName = "Namespaces"
	NetNamespaces         ResourceName = "NetNamespaces"
	Services              ResourceName = "Services"
	HostSubnets           ResourceName = "HostSubnets"
	Pods                  ResourceName = "Pods"
	EgressNetworkPolicies ResourceName = "EgressNetworkPolicies"
)

func newRegistry(osClient *osclient.Client, kClient *kclient.Client) *Registry {
	return &Registry{
		oClient: osClient,
		kClient: kClient,
	}
}

func (registry *Registry) GetSubnets() ([]osapi.HostSubnet, error) {
	hostSubnetList, err := registry.oClient.HostSubnets().List(kapi.ListOptions{})
	if err != nil {
		return nil, err
	}
	return hostSubnetList.Items, nil
}

func (registry *Registry) GetSubnet(nodeName string) (*osapi.HostSubnet, error) {
	return registry.oClient.HostSubnets().Get(nodeName)
}

func (registry *Registry) DeleteSubnet(nodeName string) error {
	return registry.oClient.HostSubnets().Delete(nodeName)
}

func (registry *Registry) CreateSubnet(nodeName, nodeIP, subnetCIDR string) (*osapi.HostSubnet, error) {
	hs := &osapi.HostSubnet{
		TypeMeta:   unversioned.TypeMeta{Kind: "HostSubnet"},
		ObjectMeta: kapi.ObjectMeta{Name: nodeName},
		Host:       nodeName,
		HostIP:     nodeIP,
		Subnet:     subnetCIDR,
	}
	return registry.oClient.HostSubnets().Create(hs)
}

func (registry *Registry) UpdateSubnet(hs *osapi.HostSubnet) (*osapi.HostSubnet, error) {
	return registry.oClient.HostSubnets().Update(hs)
}

func (registry *Registry) GetRunningPods(nodeName, namespace string) ([]kapi.Pod, error) {
	fieldSelector := fields.Set{"spec.host": nodeName}.AsSelector()
	opts := kapi.ListOptions{
		LabelSelector: labels.Everything(),
		FieldSelector: fieldSelector,
	}
	podList, err := registry.kClient.Pods(namespace).List(opts)
	if err != nil {
		return nil, err
	}

	// Filter running pods
	pods := make([]kapi.Pod, 0, len(podList.Items))
	for _, pod := range podList.Items {
		if pod.Status.Phase == kapi.PodRunning {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func (registry *Registry) GetPod(nodeName, namespace, podName string) (*kapi.Pod, error) {
	fieldSelector := fields.Set{"spec.host": nodeName}.AsSelector()
	opts := kapi.ListOptions{
		LabelSelector: labels.Everything(),
		FieldSelector: fieldSelector,
	}
	podList, err := registry.kClient.Pods(namespace).List(opts)
	if err != nil {
		return nil, err
	}

	for _, pod := range podList.Items {
		if pod.ObjectMeta.Name == podName {
			return &pod, nil
		}
	}
	return nil, nil
}

func (registry *Registry) UpdateClusterNetwork(ni *NetworkInfo) error {
	cn, err := registry.oClient.ClusterNetwork().Get(osapi.ClusterNetworkDefault)
	if err != nil {
		return err
	}
	cn.Network = ni.ClusterNetwork.String()
	cn.HostSubnetLength = ni.HostSubnetLength
	cn.ServiceNetwork = ni.ServiceNetwork.String()
	cn.PluginName = ni.PluginName
	updatedNetwork, err := registry.oClient.ClusterNetwork().Update(cn)
	if err != nil {
		return err
	}
	log.Infof("Updated ClusterNetwork %s", clusterNetworkToString(updatedNetwork))
	return nil
}

func (registry *Registry) CreateClusterNetwork(ni *NetworkInfo) error {
	cn := &osapi.ClusterNetwork{
		TypeMeta:         unversioned.TypeMeta{Kind: "ClusterNetwork"},
		ObjectMeta:       kapi.ObjectMeta{Name: osapi.ClusterNetworkDefault},
		Network:          ni.ClusterNetwork.String(),
		HostSubnetLength: ni.HostSubnetLength,
		ServiceNetwork:   ni.ServiceNetwork.String(),
		PluginName:       ni.PluginName,
	}
	updatedNetwork, err := registry.oClient.ClusterNetwork().Create(cn)
	if err != nil {
		return err
	}
	log.Infof("Created ClusterNetwork %s", clusterNetworkToString(updatedNetwork))
	return nil
}

func validateClusterNetwork(network string, hostSubnetLength uint32, serviceNetwork string, pluginName string) (*NetworkInfo, error) {
	_, cn, err := net.ParseCIDR(network)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse ClusterNetwork CIDR %s: %v", network, err)
	}

	_, sn, err := net.ParseCIDR(serviceNetwork)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse ServiceNetwork CIDR %s: %v", serviceNetwork, err)
	}

	if hostSubnetLength <= 0 || hostSubnetLength > 32 {
		return nil, fmt.Errorf("Invalid HostSubnetLength %d (not between 1 and 32)", hostSubnetLength)
	}

	return &NetworkInfo{
		ClusterNetwork:   cn,
		ServiceNetwork:   sn,
		HostSubnetLength: hostSubnetLength,
		PluginName:       pluginName,
	}, nil
}

func (registry *Registry) GetNetworkInfo() (*NetworkInfo, error) {
	// Check if we got cached network info
	if registry.NetworkInfo != nil {
		return registry.NetworkInfo, nil
	}

	cn, err := registry.oClient.ClusterNetwork().Get(osapi.ClusterNetworkDefault)
	if err != nil {
		return nil, err
	}

	registry.NetworkInfo, err = validateClusterNetwork(cn.Network, cn.HostSubnetLength, cn.ServiceNetwork, cn.PluginName)
	if err != nil {
		return nil, err
	}
	return registry.NetworkInfo, nil
}

func (registry *Registry) GetNetNamespaces() ([]osapi.NetNamespace, error) {
	netNamespaceList, err := registry.oClient.NetNamespaces().List(kapi.ListOptions{})
	if err != nil {
		return nil, err
	}
	return netNamespaceList.Items, nil
}

func (registry *Registry) GetNetNamespace(name string) (*osapi.NetNamespace, error) {
	return registry.oClient.NetNamespaces().Get(name)
}

func (registry *Registry) CreateNetNamespace(name string, id uint32) error {
	netns := &osapi.NetNamespace{
		TypeMeta:   unversioned.TypeMeta{Kind: "NetNamespace"},
		ObjectMeta: kapi.ObjectMeta{Name: name},
		NetName:    name,
		NetID:      id,
	}
	_, err := registry.oClient.NetNamespaces().Create(netns)
	return err
}

func (registry *Registry) UpdateNetNamespace(netns *osapi.NetNamespace) (*osapi.NetNamespace, error) {
	return registry.oClient.NetNamespaces().Update(netns)
}

func (registry *Registry) DeleteNetNamespace(name string) error {
	return registry.oClient.NetNamespaces().Delete(name)
}

func (registry *Registry) GetServicesForNamespace(namespace string) ([]kapi.Service, error) {
	return registry.getServices(namespace)
}

func (registry *Registry) GetServices() ([]kapi.Service, error) {
	return registry.getServices(kapi.NamespaceAll)
}

func (registry *Registry) getServices(namespace string) ([]kapi.Service, error) {
	kServList, err := registry.kClient.Services(namespace).List(kapi.ListOptions{})
	if err != nil {
		return nil, err
	}

	servList := make([]kapi.Service, 0, len(kServList.Items))
	for _, service := range kServList.Items {
		if !kapi.IsServiceIPSet(&service) {
			continue
		}
		servList = append(servList, service)
	}
	return servList, nil
}

func (registry *Registry) GetEgressNetworkPolicies() ([]osapi.EgressNetworkPolicy, error) {
	policyList, err := registry.oClient.EgressNetworkPolicies(kapi.NamespaceAll).List(kapi.ListOptions{})
	if err != nil {
		return nil, err
	}
	return policyList.Items, nil
}

// Run event queue for the given resource
func (registry *Registry) RunEventQueue(resourceName ResourceName) *oscache.EventQueue {
	var client cache.Getter
	var expectedType interface{}

	switch resourceName {
	case HostSubnets:
		expectedType = &osapi.HostSubnet{}
		client = registry.oClient
	case NetNamespaces:
		expectedType = &osapi.NetNamespace{}
		client = registry.oClient
	case Nodes:
		expectedType = &kapi.Node{}
		client = registry.kClient
	case Namespaces:
		expectedType = &kapi.Namespace{}
		client = registry.kClient
	case Services:
		expectedType = &kapi.Service{}
		client = registry.kClient
	case Pods:
		expectedType = &kapi.Pod{}
		client = registry.kClient
	case EgressNetworkPolicies:
		expectedType = &osapi.EgressNetworkPolicy{}
		client = registry.oClient
	default:
		log.Fatalf("Unknown resource %s during initialization of event queue", resourceName)
	}

	lw := cache.NewListWatchFromClient(client, strings.ToLower(string(resourceName)), kapi.NamespaceAll, fields.Everything())
	eventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	// Repopulate event queue every 30 mins
	// Existing items in the event queue will have watch.Modified event type
	cache.NewReflector(lw, expectedType, eventQueue, 30*time.Minute).Run()
	return eventQueue
}

func (registry *Registry) ValidateNodeIP(nodeIP string) error {
	if nodeIP == "" || nodeIP == "127.0.0.1" {
		return fmt.Errorf("Invalid node IP %q", nodeIP)
	}

	ni, err := registry.GetNetworkInfo()
	if err != nil {
		return fmt.Errorf("Failed to get network information: %v", err)
	}

	// Ensure each node's NodeIP is not contained by the cluster network,
	// which could cause a routing loop. (rhbz#1295486)
	ipaddr := net.ParseIP(nodeIP)
	if ipaddr == nil {
		return fmt.Errorf("Failed to parse node IP %s", nodeIP)
	}

	if ni.ClusterNetwork.Contains(ipaddr) {
		return fmt.Errorf("Node IP %s conflicts with cluster network %s", nodeIP, ni.ClusterNetwork.String())
	}

	return nil
}

func clusterNetworkToString(n *osapi.ClusterNetwork) string {
	return fmt.Sprintf("%s (network: %q, hostSubnetBits: %d, serviceNetwork: %q, pluginName: %q)", n.Name, n.Network, n.HostSubnetLength, n.ServiceNetwork, n.PluginName)
}
