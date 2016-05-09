package osdn

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

type Registry struct {
	oClient          *osclient.Client
	kClient          *kclient.Client
	serviceNetwork   *net.IPNet
	clusterNetwork   *net.IPNet
	hostSubnetLength int
}

type ResourceName string

const (
	Nodes         ResourceName = "Nodes"
	Namespaces    ResourceName = "Namespaces"
	NetNamespaces ResourceName = "NetNamespaces"
	Services      ResourceName = "Services"
	HostSubnets   ResourceName = "HostSubnets"
	Pods          ResourceName = "Pods"
)

func NewRegistry(osClient *osclient.Client, kClient *kclient.Client) *Registry {
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

func (registry *Registry) GetAllPods() ([]kapi.Pod, error) {
	podList, err := registry.kClient.Pods(kapi.NamespaceAll).List(kapi.ListOptions{})
	if err != nil {
		return nil, err
	}

	return podList.Items, nil
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

func (registry *Registry) UpdateClusterNetwork(clusterNetwork *net.IPNet, subnetLength int, serviceNetwork *net.IPNet) error {
	cn, err := registry.oClient.ClusterNetwork().Get("default")
	if err != nil {
		return err
	}
	cn.Network = clusterNetwork.String()
	cn.HostSubnetLength = subnetLength
	cn.ServiceNetwork = serviceNetwork.String()
	_, err = registry.oClient.ClusterNetwork().Update(cn)
	return err
}

func (registry *Registry) CreateClusterNetwork(clusterNetwork *net.IPNet, subnetLength int, serviceNetwork *net.IPNet) error {
	cn := &osapi.ClusterNetwork{
		TypeMeta:         unversioned.TypeMeta{Kind: "ClusterNetwork"},
		ObjectMeta:       kapi.ObjectMeta{Name: "default"},
		Network:          clusterNetwork.String(),
		HostSubnetLength: subnetLength,
		ServiceNetwork:   serviceNetwork.String(),
	}
	_, err := registry.oClient.ClusterNetwork().Create(cn)
	return err
}

func ValidateClusterNetwork(network string, hostSubnetLength int, serviceNetwork string) (*net.IPNet, int, *net.IPNet, error) {
	_, cn, err := net.ParseCIDR(network)
	if err != nil {
		return nil, -1, nil, fmt.Errorf("Failed to parse ClusterNetwork CIDR %s: %v", network, err)
	}

	_, sn, err := net.ParseCIDR(serviceNetwork)
	if err != nil {
		return nil, -1, nil, fmt.Errorf("Failed to parse ServiceNetwork CIDR %s: %v", serviceNetwork, err)
	}

	if hostSubnetLength <= 0 || hostSubnetLength > 32 {
		return nil, -1, nil, fmt.Errorf("Invalid HostSubnetLength %d (not between 1 and 32)", hostSubnetLength)
	}
	return cn, hostSubnetLength, sn, nil
}

func (registry *Registry) cacheClusterNetwork() error {
	// don't hit up the master if we have the values already
	if registry.clusterNetwork != nil && registry.serviceNetwork != nil {
		return nil
	}

	cn, err := registry.oClient.ClusterNetwork().Get("default")
	if err != nil {
		return err
	}

	registry.clusterNetwork, registry.hostSubnetLength, registry.serviceNetwork, err = ValidateClusterNetwork(cn.Network, cn.HostSubnetLength, cn.ServiceNetwork)

	return err
}

func (registry *Registry) GetNetworkInfo() (*net.IPNet, int, *net.IPNet, error) {
	if err := registry.cacheClusterNetwork(); err != nil {
		return nil, -1, nil, err
	}
	return registry.clusterNetwork, registry.hostSubnetLength, registry.serviceNetwork, nil
}

func (registry *Registry) GetClusterNetwork() (*net.IPNet, error) {
	if err := registry.cacheClusterNetwork(); err != nil {
		return nil, err
	}
	return registry.clusterNetwork, nil
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

func (registry *Registry) WriteNetNamespace(name string, id uint) error {
	netns := &osapi.NetNamespace{
		TypeMeta:   unversioned.TypeMeta{Kind: "NetNamespace"},
		ObjectMeta: kapi.ObjectMeta{Name: name},
		NetName:    name,
		NetID:      id,
	}
	_, err := registry.oClient.NetNamespaces().Create(netns)
	return err
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
