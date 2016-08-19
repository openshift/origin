package plugin

import (
	"fmt"
	"net"

	"github.com/golang/glog"

	osclient "github.com/openshift/origin/pkg/client"
	osapi "github.com/openshift/origin/pkg/sdn/api"
	"github.com/openshift/origin/pkg/sdn/plugin/api"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrs "k8s.io/kubernetes/pkg/api/errors"
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
	registry *Registry
	firewall map[string][]proxyFirewallItem

	baseEndpointsHandler pconfig.EndpointsConfigHandler
}

// Called by higher layers to create the proxy plugin instance; only used by nodes
func NewProxyPlugin(pluginName string, osClient *osclient.Client, kClient *kclient.Client) (api.FilteringEndpointsConfigHandler, error) {
	if !IsOpenShiftMultitenantNetworkPlugin(pluginName) {
		return nil, nil
	}

	return &ovsProxyPlugin{
		registry: newRegistry(osClient, kClient),
		firewall: make(map[string][]proxyFirewallItem),
	}, nil
}

func (proxy *ovsProxyPlugin) Start(baseHandler pconfig.EndpointsConfigHandler) error {
	glog.Infof("Starting multitenant SDN proxy endpoint filter")

	proxy.baseEndpointsHandler = baseHandler

	policies, err := proxy.registry.GetEgressNetworkPolicies()
	if err != nil {
		if kapierrs.IsForbidden(err) {
			// controller.go will log an error about this
			return nil
		}
		return fmt.Errorf("could not get EgressNetworkPolicies: %s", err)
	}
	for _, policy := range policies {
		proxy.updateNetworkPolicy(policy)
	}

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

func (proxy *ovsProxyPlugin) firewallBlocksIP(namespace string, ip net.IP) bool {
	for _, item := range proxy.firewall[namespace] {
		if item.net.Contains(ip) {
			return item.policy == osapi.EgressNetworkPolicyRuleDeny
		}
	}
	return false
}

func (proxy *ovsProxyPlugin) OnEndpointsUpdate(allEndpoints []kapi.Endpoints) {
	if len(proxy.firewall) == 0 {
		proxy.baseEndpointsHandler.OnEndpointsUpdate(allEndpoints)
		return
	}

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
				if !ni.ClusterNetwork.Contains(IP) && !ni.ServiceNetwork.Contains(IP) {
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
