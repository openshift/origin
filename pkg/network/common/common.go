package common

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
	"github.com/openshift/origin/pkg/util/netutils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/watch"
	kcache "k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
)

func HostSubnetToString(subnet *networkapi.HostSubnet) string {
	return fmt.Sprintf("%s (host: %q, ip: %q, subnet: %q)", subnet.Name, subnet.Host, subnet.HostIP, subnet.Subnet)
}

func ClusterNetworkToString(n *networkapi.ClusterNetwork) string {
	return fmt.Sprintf("%s (network: %q, hostSubnetBits: %d, serviceNetwork: %q, pluginName: %q)", n.Name, n.Network, n.HostSubnetLength, n.ServiceNetwork, n.PluginName)
}

func ClusterNetworkListContains(clusterNetworks []ClusterNetwork, ipaddr net.IP) (*net.IPNet, bool) {
	for _, cn := range clusterNetworks {
		if cn.ClusterCIDR.Contains(ipaddr) {
			return cn.ClusterCIDR, true
		}
	}
	return nil, false
}

type NetworkInfo struct {
	ClusterNetworks []ClusterNetwork
	ServiceNetwork  *net.IPNet
}

type ClusterNetwork struct {
	ClusterCIDR      *net.IPNet
	HostSubnetLength uint32
}

func ParseNetworkInfo(clusterNetwork []networkapi.ClusterNetworkEntry, serviceNetwork string) (*NetworkInfo, error) {
	var cns []ClusterNetwork

	for _, entry := range clusterNetwork {
		cidr, err := netutils.ParseCIDRMask(entry.CIDR)
		if err != nil {
			_, cidr, err = net.ParseCIDR(entry.CIDR)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ClusterNetwork CIDR %s: %v", entry.CIDR, err)
			}
			glog.Errorf("Configured clusterNetworks value %q is invalid; treating it as %q", entry.CIDR, cidr.String())
		}
		cns = append(cns, ClusterNetwork{ClusterCIDR: cidr, HostSubnetLength: entry.HostSubnetLength})
	}

	sn, err := netutils.ParseCIDRMask(serviceNetwork)
	if err != nil {
		_, sn, err = net.ParseCIDR(serviceNetwork)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ServiceNetwork CIDR %s: %v", serviceNetwork, err)
		}
		glog.Errorf("Configured serviceNetworkCIDR value %q is invalid; treating it as %q", serviceNetwork, sn.String())
	}

	return &NetworkInfo{
		ClusterNetworks: cns,
		ServiceNetwork:  sn,
	}, nil
}

func (ni *NetworkInfo) ValidateNodeIP(nodeIP string) error {
	if nodeIP == "" || nodeIP == "127.0.0.1" {
		return fmt.Errorf("invalid node IP %q", nodeIP)
	}

	// Ensure each node's NodeIP is not contained by the cluster network,
	// which could cause a routing loop. (rhbz#1295486)
	ipaddr := net.ParseIP(nodeIP)
	if ipaddr == nil {
		return fmt.Errorf("failed to parse node IP %s", nodeIP)
	}

	if conflictingCIDR, found := ClusterNetworkListContains(ni.ClusterNetworks, ipaddr); found {
		return fmt.Errorf("node IP %s conflicts with cluster network %s", nodeIP, conflictingCIDR.String())
	}
	if ni.ServiceNetwork.Contains(ipaddr) {
		return fmt.Errorf("node IP %s conflicts with service network %s", nodeIP, ni.ServiceNetwork.String())
	}

	return nil
}

func (ni *NetworkInfo) CheckHostNetworks(hostIPNets []*net.IPNet) error {
	errList := []error{}
	for _, ipNet := range hostIPNets {
		for _, clusterNetwork := range ni.ClusterNetworks {
			if configapi.CIDRsOverlap(ipNet.String(), clusterNetwork.ClusterCIDR.String()) {
				errList = append(errList, fmt.Errorf("cluster IP: %s conflicts with host network: %s", clusterNetwork.ClusterCIDR.IP.String(), ipNet.String()))
			}
		}
		if configapi.CIDRsOverlap(ipNet.String(), ni.ServiceNetwork.String()) {
			errList = append(errList, fmt.Errorf("service IP: %s conflicts with host network: %s", ni.ServiceNetwork.String(), ipNet.String()))
		}
	}
	return kerrors.NewAggregate(errList)
}

func (ni *NetworkInfo) CheckClusterObjects(subnets []networkapi.HostSubnet, pods []kapi.Pod, services []kapi.Service) error {
	var errList []error

	for _, subnet := range subnets {
		subnetIP, _, _ := net.ParseCIDR(subnet.Subnet)
		if subnetIP == nil {
			errList = append(errList, fmt.Errorf("failed to parse network address: %s", subnet.Subnet))
		} else if _, contains := ClusterNetworkListContains(ni.ClusterNetworks, subnetIP); !contains {
			errList = append(errList, fmt.Errorf("existing node subnet: %s is not part of any cluster network CIDR", subnet.Subnet))
		}
		if len(errList) >= 10 {
			break
		}
	}
	for _, pod := range pods {
		if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.HostNetwork {
			continue
		}
		if _, contains := ClusterNetworkListContains(ni.ClusterNetworks, net.ParseIP(pod.Status.PodIP)); !contains && pod.Status.PodIP != "" {
			errList = append(errList, fmt.Errorf("existing pod %s:%s with IP %s is not part of cluster network", pod.Namespace, pod.Name, pod.Status.PodIP))
			if len(errList) >= 10 {
				break
			}
		}
	}
	for _, svc := range services {
		svcIP := net.ParseIP(svc.Spec.ClusterIP)
		if svcIP != nil && !ni.ServiceNetwork.Contains(svcIP) {
			errList = append(errList, fmt.Errorf("existing service %s:%s with IP %s is not part of service network %s", svc.Namespace, svc.Name, svc.Spec.ClusterIP, ni.ServiceNetwork.String()))
			if len(errList) >= 10 {
				break
			}
		}
	}

	if len(errList) >= 10 {
		errList = append(errList, fmt.Errorf("too many errors... truncating"))
	}
	return kerrors.NewAggregate(errList)
}

func GetNetworkInfo(networkClient networkclient.Interface) (*NetworkInfo, error) {
	cn, err := networkClient.Network().ClusterNetworks().Get(networkapi.ClusterNetworkDefault, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return ParseNetworkInfo(cn.ClusterNetworks, cn.ServiceNetwork)
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
	NetworkPolicies       ResourceName = "NetworkPolicies"
)

func newEventQueue(client kcache.Getter, resourceName ResourceName, expectedType interface{}, namespace string) *EventQueue {
	rn := strings.ToLower(string(resourceName))
	lw := kcache.NewListWatchFromClient(client, rn, namespace, fields.Everything())
	eventQueue := NewEventQueue(DeletionHandlingMetaNamespaceKeyFunc)
	// Repopulate event queue every 30 mins
	// Existing items in the event queue will have watch.Modified event type
	kcache.NewReflector(lw, expectedType, eventQueue, 30*time.Minute).Run()
	return eventQueue
}

// Run event queue for the given resource. The 'process' function is called
// repeatedly with each available cache.Delta that describes state changes
// to an object. If the process function returns an error queued changes
// for that object are dropped but processing continues with the next available
// object's cache.Deltas.  The error is logged with call stack information.
//
// NOTE: this function will handle DeletedFinalStateUnknown delta objects
// automatically, which may not always be what you want since the now-deleted
// object may be stale.
func RunEventQueue(client kcache.Getter, resourceName ResourceName, process ProcessEventFunc) {
	var expectedType interface{}

	switch resourceName {
	case HostSubnets:
		expectedType = &networkapi.HostSubnet{}
	case NetNamespaces:
		expectedType = &networkapi.NetNamespace{}
	case EgressNetworkPolicies:
		expectedType = &networkapi.EgressNetworkPolicy{}
	case NetworkPolicies:
		expectedType = &extensions.NetworkPolicy{}
	default:
		glog.Fatalf("Unknown resource %s during initialization of event queue", resourceName)
	}

	eventQueue := newEventQueue(client, resourceName, expectedType, metav1.NamespaceAll)
	for {
		eventQueue.Pop(process, expectedType)
	}
}

// RegisterSharedInformerEventHandlers registers addOrUpdateFunc and delFunc event handlers with
// kubernetes shared informers for the given resource name.
func RegisterSharedInformerEventHandlers(kubeInformers kinternalinformers.SharedInformerFactory,
	addOrUpdateFunc func(interface{}, interface{}, watch.EventType),
	delFunc func(interface{}), resourceName ResourceName) {

	var expectedObjType interface{}
	var informer kcache.SharedIndexInformer

	internalVersion := kubeInformers.Core().InternalVersion()

	switch resourceName {
	case Nodes:
		informer = internalVersion.Nodes().Informer()
		expectedObjType = &kapi.Node{}
	case Namespaces:
		informer = internalVersion.Namespaces().Informer()
		expectedObjType = &kapi.Namespace{}
	case Services:
		informer = internalVersion.Services().Informer()
		expectedObjType = &kapi.Service{}
	case Pods:
		informer = internalVersion.Pods().Informer()
		expectedObjType = &kapi.Pod{}
	default:
		glog.Errorf("Unknown resource name: %s", resourceName)
		return
	}

	informer.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			addOrUpdateFunc(obj, nil, watch.Added)
		},
		UpdateFunc: func(old, cur interface{}) {
			addOrUpdateFunc(cur, old, watch.Modified)
		},
		DeleteFunc: func(obj interface{}) {
			if reflect.TypeOf(expectedObjType) != reflect.TypeOf(obj) {
				tombstone, ok := obj.(kcache.DeletedFinalStateUnknown)
				if !ok {
					glog.Errorf("Couldn't get object from tombstone: %+v", obj)
					return
				}

				obj = tombstone.Obj
				if reflect.TypeOf(expectedObjType) != reflect.TypeOf(obj) {
					glog.Errorf("Tombstone contained object that is not a %s: %+v", resourceName, obj)
					return
				}
			}
			delFunc(obj)
		},
	})
}
