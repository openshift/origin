package plugin

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/apis/network"
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

func hostSubnetToString(subnet *osapi.HostSubnet) string {
	return fmt.Sprintf("%s (host: %q, ip: %q, subnet: %q)", subnet.Name, subnet.Host, subnet.HostIP, subnet.Subnet)
}

func clusterNetworkToString(n *osapi.ClusterNetwork) string {
	return fmt.Sprintf("%s (network: %q, hostSubnetBits: %d, serviceNetwork: %q, pluginName: %q)", n.Name, n.Network, n.HostSubnetLength, n.ServiceNetwork, n.PluginName)
}

type NetworkInfo struct {
	ClusterNetwork *net.IPNet
	ServiceNetwork *net.IPNet
}

func parseNetworkInfo(clusterNetwork string, serviceNetwork string) (*NetworkInfo, error) {
	cn, err := netutils.ParseCIDRMask(clusterNetwork)
	if err != nil {
		_, cn, err := net.ParseCIDR(clusterNetwork)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ClusterNetwork CIDR %s: %v", clusterNetwork, err)
		}
		glog.Errorf("Configured clusterNetworkCIDR value %q is invalid; treating it as %q", clusterNetwork, cn.String())
	}
	sn, err := netutils.ParseCIDRMask(serviceNetwork)
	if err != nil {
		_, sn, err := net.ParseCIDR(serviceNetwork)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ServiceNetwork CIDR %s: %v", serviceNetwork, err)
		}
		glog.Errorf("Configured serviceNetworkCIDR value %q is invalid; treating it as %q", serviceNetwork, sn.String())
	}

	return &NetworkInfo{
		ClusterNetwork: cn,
		ServiceNetwork: sn,
	}, nil
}

func (ni *NetworkInfo) validateNodeIP(nodeIP string) error {
	if nodeIP == "" || nodeIP == "127.0.0.1" {
		return fmt.Errorf("invalid node IP %q", nodeIP)
	}

	// Ensure each node's NodeIP is not contained by the cluster network,
	// which could cause a routing loop. (rhbz#1295486)
	ipaddr := net.ParseIP(nodeIP)
	if ipaddr == nil {
		return fmt.Errorf("failed to parse node IP %s", nodeIP)
	}

	if ni.ClusterNetwork.Contains(ipaddr) {
		return fmt.Errorf("node IP %s conflicts with cluster network %s", nodeIP, ni.ClusterNetwork.String())
	}
	if ni.ServiceNetwork.Contains(ipaddr) {
		return fmt.Errorf("node IP %s conflicts with service network %s", nodeIP, ni.ServiceNetwork.String())
	}

	return nil
}

func (ni *NetworkInfo) checkHostNetworks(hostIPNets []*net.IPNet) error {
	errList := []error{}
	for _, ipNet := range hostIPNets {
		if ipNet.Contains(ni.ClusterNetwork.IP) {
			errList = append(errList, fmt.Errorf("cluster IP: %s conflicts with host network: %s", ni.ClusterNetwork.IP.String(), ipNet.String()))
		}
		if ni.ClusterNetwork.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("host network with IP: %s conflicts with cluster network: %s", ipNet.IP.String(), ni.ClusterNetwork.String()))
		}
		if ipNet.Contains(ni.ServiceNetwork.IP) {
			errList = append(errList, fmt.Errorf("service IP: %s conflicts with host network: %s", ni.ServiceNetwork.String(), ipNet.String()))
		}
		if ni.ServiceNetwork.Contains(ipNet.IP) {
			errList = append(errList, fmt.Errorf("host network with IP: %s conflicts with service network: %s", ipNet.IP.String(), ni.ServiceNetwork.String()))
		}
	}
	return kerrors.NewAggregate(errList)
}

func (ni *NetworkInfo) checkClusterObjects(subnets []osapi.HostSubnet, pods []kapi.Pod, services []kapi.Service) error {
	var errList []error

	for _, subnet := range subnets {
		subnetIP, _, _ := net.ParseCIDR(subnet.Subnet)
		if subnetIP == nil {
			errList = append(errList, fmt.Errorf("failed to parse network address: %s", subnet.Subnet))
		} else if !ni.ClusterNetwork.Contains(subnetIP) {
			errList = append(errList, fmt.Errorf("existing node subnet: %s is not part of cluster network: %s", subnet.Subnet, ni.ClusterNetwork.String()))
		}
		if len(errList) >= 10 {
			break
		}
	}
	for _, pod := range pods {
		if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.HostNetwork {
			continue
		}
		if pod.Status.PodIP != "" && !ni.ClusterNetwork.Contains(net.ParseIP(pod.Status.PodIP)) {
			errList = append(errList, fmt.Errorf("existing pod %s:%s with IP %s is not part of cluster network %s", pod.Namespace, pod.Name, pod.Status.PodIP, ni.ClusterNetwork.String()))
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

func getNetworkInfo(osClient *osclient.Client) (*NetworkInfo, error) {
	cn, err := osClient.ClusterNetwork().Get(osapi.ClusterNetworkDefault, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return parseNetworkInfo(cn.Network, cn.ServiceNetwork)
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
		expectedType = &osapi.HostSubnet{}
	case NetNamespaces:
		expectedType = &osapi.NetNamespace{}
	case EgressNetworkPolicies:
		expectedType = &osapi.EgressNetworkPolicy{}
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
