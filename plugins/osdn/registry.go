package osdn

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	log "github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	osapi "github.com/openshift/origin/pkg/sdn/api"
)

type Registry struct {
	oClient          *osclient.Client
	kClient          *kclient.Client
	podsByIP         map[string]*kapi.Pod
	podTrackingLock  sync.Mutex
	serviceNetwork   *net.IPNet
	clusterNetwork   *net.IPNet
	hostSubnetLength int

	// These are only set if SetBaseEndpointsHandler() has been called
	baseEndpointsHandler pconfig.EndpointsConfigHandler
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
		oClient:  osClient,
		kClient:  kClient,
		podsByIP: make(map[string]*kapi.Pod),
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

func (registry *Registry) PopulatePodsByIP() error {
	podList, err := registry.kClient.Pods(kapi.NamespaceAll).List(kapi.ListOptions{})
	if err != nil {
		return err
	}

	for _, pod := range podList.Items {
		registry.TrackPod(&pod)
	}
	return nil
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

// FilteringEndpointsConfigHandler implementation
func (registry *Registry) SetBaseEndpointsHandler(base pconfig.EndpointsConfigHandler) {
	registry.baseEndpointsHandler = base
}

func (registry *Registry) OnEndpointsUpdate(allEndpoints []kapi.Endpoints) {
	clusterNetwork, _, serviceNetwork, err := registry.GetNetworkInfo()
	if err != nil {
		log.Warningf("Error fetching cluster network: %v", err)
		return
	}

	filteredEndpoints := make([]kapi.Endpoints, 0, len(allEndpoints))
EndpointLoop:
	for _, ep := range allEndpoints {
		ns := ep.ObjectMeta.Namespace
		for _, ss := range ep.Subsets {
			for _, addr := range ss.Addresses {
				IP := net.ParseIP(addr.IP)
				if serviceNetwork.Contains(IP) {
					log.Warningf("Service '%s' in namespace '%s' has an Endpoint inside the service network (%s)", ep.ObjectMeta.Name, ns, addr.IP)
					continue EndpointLoop
				}
				if clusterNetwork.Contains(IP) {
					podInfo, ok := registry.getTrackedPod(addr.IP)
					if !ok {
						log.Warningf("Service '%s' in namespace '%s' has an Endpoint pointing to non-existent pod (%s)", ep.ObjectMeta.Name, ns, addr.IP)
						continue EndpointLoop
					}
					if podInfo.ObjectMeta.Namespace != ns {
						log.Warningf("Service '%s' in namespace '%s' has an Endpoint pointing to pod %s in namespace '%s'", ep.ObjectMeta.Name, ns, addr.IP, podInfo.ObjectMeta.Namespace)
						continue EndpointLoop
					}
				}
			}
		}
		filteredEndpoints = append(filteredEndpoints, ep)
	}

	registry.baseEndpointsHandler.OnEndpointsUpdate(filteredEndpoints)
}

func (registry *Registry) getTrackedPod(ip string) (*kapi.Pod, bool) {
	registry.podTrackingLock.Lock()
	defer registry.podTrackingLock.Unlock()

	pod, ok := registry.podsByIP[ip]
	return pod, ok
}

func (registry *Registry) TrackPod(pod *kapi.Pod) {
	if pod.Status.PodIP == "" {
		return
	}

	registry.podTrackingLock.Lock()
	defer registry.podTrackingLock.Unlock()
	podInfo, ok := registry.podsByIP[pod.Status.PodIP]

	if pod.Status.Phase == kapi.PodPending || pod.Status.Phase == kapi.PodRunning {
		// When a pod hits one of the states where the IP is in use then
		// we need to add it to our IP -> namespace tracker.  There _should_ be no
		// other entries for the IP if we caught all of the right messages, so warn
		// if we see one, but clobber it anyway since the IPAM
		// should ensure that each IP is uniquely assigned to a pod (when running)
		if ok && podInfo.UID != pod.UID {
			log.Warningf("IP '%s' was marked as used by namespace '%s' (pod '%s')... updating to namespace '%s' (pod '%s')",
				pod.Status.PodIP, podInfo.Namespace, podInfo.UID, pod.ObjectMeta.Namespace, pod.UID)
		}

		registry.podsByIP[pod.Status.PodIP] = pod
	} else if ok && podInfo.UID == pod.UID {
		// If the UIDs match, then this pod is moving to a state that indicates it is not running
		// so we need to remove it from the cache
		delete(registry.podsByIP, pod.Status.PodIP)
	}

	return
}

func (registry *Registry) UnTrackPod(pod *kapi.Pod) {
	registry.podTrackingLock.Lock()
	defer registry.podTrackingLock.Unlock()

	// Only delete if the pod ID is the one we are tracking (in case there is a failed or complete
	// pod lying around that gets deleted while there is a running pod with the same IP)
	if podInfo, ok := registry.podsByIP[pod.Status.PodIP]; ok && podInfo.UID == pod.UID {
		delete(registry.podsByIP, pod.Status.PodIP)
	}

	return
}
