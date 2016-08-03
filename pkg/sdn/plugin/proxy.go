package plugin

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/plugin/api"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"
)

type proxyFirewallItem struct {
	policy osapi.EgressNetworkPolicyRuleType
	net    *net.IPNet
}

type ovsProxyPlugin struct {
	registry  *Registry
	podsByIP  map[string]*kapi.Pod
	podsMutex sync.Mutex
	firewall  map[string][]proxyFirewallItem

	baseEndpointsHandler pconfig.EndpointsConfigHandler
}

// Called by higher layers to create the proxy plugin instance; only used by nodes
func NewProxyPlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client) (api.FilteringEndpointsConfigHandler, error) {
	if !IsOpenShiftMultitenantNetworkPlugin(pluginName) {
		return nil, nil
	}

	return &ovsProxyPlugin{
		registry: newRegistry(osClient, kClient),
		podsByIP: make(map[string]*kapi.Pod),
		firewall: make(map[string][]proxyFirewallItem),
	}, nil
}

func (proxy *ovsProxyPlugin) Start(baseHandler pconfig.EndpointsConfigHandler) error {
	glog.Infof("Starting multitenant SDN proxy endpoint filter")

	proxy.baseEndpointsHandler = baseHandler

	// Populate pod info map synchronously so that kube proxy can filter endpoints to support isolation
	pods, err := proxy.registry.GetAllPods()
	if err != nil {
		return err
	}

	policies, err := proxy.registry.GetEgressNetworkPolicies()
	if err != nil {
		return fmt.Errorf("Could not get EgressNetworkPolicies: %s", err)
	}
	for _, policy := range policies {
		proxy.updateNetworkPolicy(policy)
	}

	for _, pod := range pods {
		proxy.trackPod(&pod)
	}

	go utilwait.Forever(proxy.watchPods, 0)
	go utilwait.Forever(proxy.watchEgressNetworkPolicies, 0)

	return nil
}

func (proxy *ovsProxyPlugin) watchEgressNetworkPolicies() {
	eventQueue := proxy.registry.RunEventQueue(EgressNetworkPolicies)

	for {
		eventType, obj, err := eventQueue.Pop()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("EventQueue failed for EgressNetworkPolicy: %v", err))
			return
		}
		policy := obj.(*osapi.EgressNetworkPolicy)
		if eventType == watch.Deleted {
			policy.Spec.Egress = nil
		}
		proxy.updateNetworkPolicy(*policy)
		// FIXME: poke the endpoint-syncer somehow...
	}
}

func (proxy *ovsProxyPlugin) updateNetworkPolicy(policy osapi.EgressNetworkPolicy) {
	firewall := make([]proxyFirewallItem, len(policy.Spec.Egress))
	for i, rule := range policy.Spec.Egress {
		_, cidr, err := net.ParseCIDR(rule.To.CIDRSelector)
		if err != nil {
			// should have been caught by validation
			glog.Errorf("Illegal CIDR value %q in EgressNetworkPolicy rule", rule.To.CIDRSelector)
			return
		}
		firewall[i] = proxyFirewallItem{rule.Type, cidr}
	}

	if len(firewall) > 0 {
		proxy.firewall[policy.Namespace] = firewall
	} else {
		delete(proxy.firewall, policy.Namespace)
	}
}

func (proxy *ovsProxyPlugin) watchPods() {
	eventQueue := proxy.registry.RunEventQueue(Pods)

	for {
		eventType, obj, err := eventQueue.Pop()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("EventQueue failed for pods: %v", err))
			return
		}
		pod := obj.(*kapi.Pod)

		glog.V(5).Infof("Watch %s event for Pod %q", strings.Title(string(eventType)), pod.ObjectMeta.Name)
		switch eventType {
		case watch.Added, watch.Modified:
			proxy.trackPod(pod)
		case watch.Deleted:
			proxy.unTrackPod(pod)
		}
	}
}

func (proxy *ovsProxyPlugin) getTrackedPod(ip string) (*kapi.Pod, bool) {
	proxy.podsMutex.Lock()
	defer proxy.podsMutex.Unlock()

	pod, ok := proxy.podsByIP[ip]
	return pod, ok
}

func (proxy *ovsProxyPlugin) trackPod(pod *kapi.Pod) {
	if pod.Status.PodIP == "" {
		return
	}

	proxy.podsMutex.Lock()
	defer proxy.podsMutex.Unlock()
	podInfo, ok := proxy.podsByIP[pod.Status.PodIP]

	if pod.Status.Phase == kapi.PodPending || pod.Status.Phase == kapi.PodRunning {
		// When a pod hits one of the states where the IP is in use then
		// we need to add it to our IP -> namespace tracker.  There _should_ be no
		// other entries for the IP if we caught all of the right messages, so warn
		// if we see one, but clobber it anyway since the IPAM
		// should ensure that each IP is uniquely assigned to a pod (when running)
		if ok && podInfo.UID != pod.UID {
			glog.Warningf("IP '%s' was marked as used by namespace '%s' (pod '%s')... updating to namespace '%s' (pod '%s')",
				pod.Status.PodIP, podInfo.Namespace, podInfo.UID, pod.ObjectMeta.Namespace, pod.UID)
		}

		proxy.podsByIP[pod.Status.PodIP] = pod
	} else if ok && podInfo.UID == pod.UID {
		// If the UIDs match, then this pod is moving to a state that indicates it is not running
		// so we need to remove it from the cache
		delete(proxy.podsByIP, pod.Status.PodIP)
	}
}

func (proxy *ovsProxyPlugin) unTrackPod(pod *kapi.Pod) {
	proxy.podsMutex.Lock()
	defer proxy.podsMutex.Unlock()

	// Only delete if the pod ID is the one we are tracking (in case there is a failed or complete
	// pod lying around that gets deleted while there is a running pod with the same IP)
	if podInfo, ok := proxy.podsByIP[pod.Status.PodIP]; ok && podInfo.UID == pod.UID {
		delete(proxy.podsByIP, pod.Status.PodIP)
	}
}

func (proxy *ovsProxyPlugin) firewallBlocksIP(namespace string, ip net.IP) bool {
	for _, item := range proxy.firewall[namespace] {
		if item.net.Contains(ip) {
			return item.policy == osapi.EgressNetworkPolicyRuleDeny
		}
	}
	return false
}

func (proxy *ovsProxyPlugin) OnEndpointsUpdate(allEndpoints []kapi.Endpoints) {
	ni, err := proxy.registry.GetNetworkInfo()
	if err != nil {
		glog.Warningf("Error fetching network information: %v", err)
		return
	}

	filteredEndpoints := make([]kapi.Endpoints, 0, len(allEndpoints))

EndpointLoop:
	for _, ep := range allEndpoints {
		ns := ep.ObjectMeta.Namespace
		for _, ss := range ep.Subsets {
			for _, addr := range ss.Addresses {
				IP := net.ParseIP(addr.IP)
				if ni.ServiceNetwork.Contains(IP) {
					glog.Warningf("Service '%s' in namespace '%s' has an Endpoint inside the service network (%s)", ep.ObjectMeta.Name, ns, addr.IP)
					continue EndpointLoop
				} else if ni.ClusterNetwork.Contains(IP) {
					podInfo, ok := proxy.getTrackedPod(addr.IP)
					if !ok {
						glog.Warningf("Service '%s' in namespace '%s' has an Endpoint pointing to non-existent pod (%s)", ep.ObjectMeta.Name, ns, addr.IP)
						continue EndpointLoop
					}
					if podInfo.ObjectMeta.Namespace != ns {
						glog.Warningf("Service '%s' in namespace '%s' has an Endpoint pointing to pod %s in namespace '%s'", ep.ObjectMeta.Name, ns, addr.IP, podInfo.ObjectMeta.Namespace)
						continue EndpointLoop
					}
				} else {
					if proxy.firewallBlocksIP(ns, IP) {
						glog.Warningf("Service '%s' in namespace '%s' has an Endpoint pointing to firewalled destination (%s)", ep.ObjectMeta.Name, ns, addr.IP)
						continue EndpointLoop
					}
				}
			}
		}
		filteredEndpoints = append(filteredEndpoints, ep)
	}

	proxy.baseEndpointsHandler.OnEndpointsUpdate(filteredEndpoints)
}
