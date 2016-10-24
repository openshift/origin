package plugin

import (
	"fmt"
	"net"
	"sync"

	"github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
)

type proxyFirewallItem struct {
	policy osapi.EgressNetworkPolicyRuleType
	net    *net.IPNet
}

type OsdnProxy struct {
	kClient              *kclient.Client
	osClient             *osclient.Client
	networkInfo          *NetworkInfo
	baseEndpointsHandler pconfig.EndpointsConfigHandler

	lock         sync.Mutex
	firewall     map[string][]proxyFirewallItem
	allEndpoints []kapi.Endpoints
}

// Called by higher layers to create the proxy plugin instance; only used by nodes
func NewProxyPlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client) (*OsdnProxy, error) {
	if !osapi.IsOpenShiftMultitenantNetworkPlugin(pluginName) {
		return nil, nil
	}

	return &OsdnProxy{
		kClient:  kClient,
		osClient: osClient,
		firewall: make(map[string][]proxyFirewallItem),
	}, nil
}

func (proxy *OsdnProxy) Start(baseHandler pconfig.EndpointsConfigHandler) error {
	glog.Infof("Starting multitenant SDN proxy endpoint filter")

	var err error
	proxy.networkInfo, err = getNetworkInfo(proxy.osClient)
	if err != nil {
		return fmt.Errorf("could not get network info: %s", err)
	}
	proxy.baseEndpointsHandler = baseHandler

	policies, err := proxy.osClient.EgressNetworkPolicies(kapi.NamespaceAll).List(kapi.ListOptions{})
	if err != nil {
		return fmt.Errorf("Could not get EgressNetworkPolicies: %s", err)
	}
	for _, policy := range policies.Items {
		proxy.updateNetworkPolicy(policy)
	}

	go utilwait.Forever(proxy.watchEgressNetworkPolicies, 0)
	return nil
}

func (proxy *OsdnProxy) watchEgressNetworkPolicies() {
	RunEventQueue(proxy.osClient, EgressNetworkPolicies, func(delta cache.Delta) error {
		policy := delta.Object.(*osapi.EgressNetworkPolicy)
		if delta.Type == cache.Deleted {
			policy.Spec.Egress = nil
		}

		func() {
			proxy.lock.Lock()
			defer proxy.lock.Unlock()
			proxy.updateNetworkPolicy(*policy)
			if proxy.allEndpoints != nil {
				proxy.updateEndpoints()
			}
		}()
		return nil
	})
}

func (proxy *OsdnProxy) updateNetworkPolicy(policy osapi.EgressNetworkPolicy) {
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

func (proxy *OsdnProxy) firewallBlocksIP(namespace string, ip net.IP) bool {
	for _, item := range proxy.firewall[namespace] {
		if item.net.Contains(ip) {
			return item.policy == osapi.EgressNetworkPolicyRuleDeny
		}
	}
	return false
}

func (proxy *OsdnProxy) OnEndpointsUpdate(allEndpoints []kapi.Endpoints) {
	proxy.lock.Lock()
	defer proxy.lock.Unlock()
	proxy.allEndpoints = allEndpoints
	proxy.updateEndpoints()
}

func (proxy *OsdnProxy) updateEndpoints() {
	if len(proxy.firewall) == 0 {
		proxy.baseEndpointsHandler.OnEndpointsUpdate(proxy.allEndpoints)
		return
	}

	filteredEndpoints := make([]kapi.Endpoints, 0, len(proxy.allEndpoints))

EndpointLoop:
	for _, ep := range proxy.allEndpoints {
		ns := ep.ObjectMeta.Namespace
		for _, ss := range ep.Subsets {
			for _, addr := range ss.Addresses {
				IP := net.ParseIP(addr.IP)
				if !proxy.networkInfo.ClusterNetwork.Contains(IP) && !proxy.networkInfo.ServiceNetwork.Contains(IP) {
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
