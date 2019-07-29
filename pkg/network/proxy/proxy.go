// +build linux

package proxy

import (
	"fmt"
	"net"
	"sync"

	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	kubeproxy "k8s.io/kubernetes/pkg/proxy"

	networkv1 "github.com/openshift/api/network/v1"
	networkclient "github.com/openshift/client-go/network/clientset/versioned"
	networkinformers "github.com/openshift/client-go/network/informers/externalversions"
	"github.com/openshift/origin/pkg/network"
	"github.com/openshift/origin/pkg/network/common"
)

// EndpointsConfigHandler is an abstract interface of objects which receive update notifications for the set of endpoints.
type EndpointsConfigHandler interface {
	// OnEndpointsUpdate gets called when endpoints configuration is changed for a given
	// service on any of the configuration sources. An example is when a new
	// service comes up, or when containers come up or down for an existing service.
	OnEndpointsUpdate(endpoints []*corev1.Endpoints)
}

type firewallItem struct {
	ruleType networkv1.EgressNetworkPolicyRuleType
	net      *net.IPNet
}

type proxyFirewallItem struct {
	namespaceFirewalls map[ktypes.UID][]firewallItem
	activePolicy       *ktypes.UID
}

type proxyEndpoints struct {
	endpoints *corev1.Endpoints
	blocked   bool
}

type OsdnProxy struct {
	kClient          kubernetes.Interface
	networkClient    networkclient.Interface
	networkInformers networkinformers.SharedInformerFactory
	networkInfo      *common.NetworkInfo
	egressDNS        *common.EgressDNS
	baseProxy        kubeproxy.ProxyProvider

	lock         sync.Mutex
	firewall     map[string]*proxyFirewallItem
	allEndpoints map[ktypes.UID]*proxyEndpoints

	idLock sync.Mutex
	ids    map[string]uint32
}

// Called by higher layers to create the proxy plugin instance; only used by nodes
func New(pluginName string, networkClient networkclient.Interface, kClient kubernetes.Interface,
	networkInformers networkinformers.SharedInformerFactory) (*OsdnProxy, error) {
	return &OsdnProxy{
		kClient:          kClient,
		networkClient:    networkClient,
		networkInformers: networkInformers,
		ids:              make(map[string]uint32),
		egressDNS:        common.NewEgressDNS(),
		firewall:         make(map[string]*proxyFirewallItem),
		allEndpoints:     make(map[ktypes.UID]*proxyEndpoints),
	}, nil
}

func (proxy *OsdnProxy) Start(proxier kubeproxy.ProxyProvider) error {
	klog.Infof("Starting multitenant SDN proxy endpoint filter")

	var err error
	proxy.networkInfo, err = common.GetNetworkInfo(proxy.networkClient)
	if err != nil {
		return fmt.Errorf("could not get network info: %s", err)
	}
	proxy.baseProxy = proxier

	policies, err := proxy.networkClient.NetworkV1().EgressNetworkPolicies(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("could not get EgressNetworkPolicies: %s", err)
	}

	proxy.lock.Lock()
	defer proxy.lock.Unlock()

	for _, policy := range policies.Items {
		proxy.egressDNS.Add(policy)
		proxy.updateEgressNetworkPolicy(policy)
	}

	go utilwait.Forever(proxy.syncEgressDNSProxyFirewall, 0)
	proxy.watchEgressNetworkPolicies()
	proxy.watchNetNamespaces()
	return nil
}

func (proxy *OsdnProxy) updateEgressNetworkPolicyLocked(policy networkv1.EgressNetworkPolicy) {
	proxy.lock.Lock()
	defer proxy.lock.Unlock()
	proxy.updateEgressNetworkPolicy(policy)
}

func (proxy *OsdnProxy) watchEgressNetworkPolicies() {
	funcs := common.InformerFuncs(&networkv1.EgressNetworkPolicy{}, proxy.handleAddOrUpdateEgressNetworkPolicy, proxy.handleDeleteEgressNetworkPolicy)
	proxy.networkInformers.Network().V1().EgressNetworkPolicies().Informer().AddEventHandler(funcs)
}

func (proxy *OsdnProxy) handleAddOrUpdateEgressNetworkPolicy(obj, _ interface{}, eventType watch.EventType) {
	policy := obj.(*networkv1.EgressNetworkPolicy)
	klog.V(5).Infof("Watch %s event for EgressNetworkPolicy %s/%s", eventType, policy.Namespace, policy.Name)

	proxy.egressDNS.Delete(*policy)
	proxy.egressDNS.Add(*policy)

	proxy.lock.Lock()
	defer proxy.lock.Unlock()
	proxy.updateEgressNetworkPolicy(*policy)
}

func (proxy *OsdnProxy) handleDeleteEgressNetworkPolicy(obj interface{}) {
	policy := obj.(*networkv1.EgressNetworkPolicy)
	klog.V(5).Infof("Watch %s event for EgressNetworkPolicy %s/%s", watch.Deleted, policy.Namespace, policy.Name)

	proxy.egressDNS.Delete(*policy)
	policy.Spec.Egress = nil

	proxy.updateEgressNetworkPolicyLocked(*policy)
}

func (proxy *OsdnProxy) watchNetNamespaces() {
	funcs := common.InformerFuncs(&networkv1.NetNamespace{}, proxy.handleAddOrUpdateNetNamespace, proxy.handleDeleteNetNamespace)
	proxy.networkInformers.Network().V1().NetNamespaces().Informer().AddEventHandler(funcs)
}

func (proxy *OsdnProxy) handleAddOrUpdateNetNamespace(obj, _ interface{}, eventType watch.EventType) {
	netns := obj.(*networkv1.NetNamespace)
	klog.V(5).Infof("Watch %s event for NetNamespace %q", eventType, netns.Name)

	proxy.idLock.Lock()
	defer proxy.idLock.Unlock()
	proxy.ids[netns.Name] = netns.NetID
}

func (proxy *OsdnProxy) handleDeleteNetNamespace(obj interface{}) {
	netns := obj.(*networkv1.NetNamespace)
	klog.V(5).Infof("Watch %s event for NetNamespace %q", watch.Deleted, netns.Name)

	proxy.idLock.Lock()
	defer proxy.idLock.Unlock()
	delete(proxy.ids, netns.Name)
}

func (proxy *OsdnProxy) isNamespaceGlobal(ns string) bool {
	proxy.idLock.Lock()
	defer proxy.idLock.Unlock()

	if proxy.ids[ns] == network.GlobalVNID {
		return true
	}
	return false
}

func (proxy *OsdnProxy) updateEgressNetworkPolicy(policy networkv1.EgressNetworkPolicy) {
	ns := policy.Namespace
	if proxy.isNamespaceGlobal(ns) {
		// Firewall not allowed for global namespaces
		utilruntime.HandleError(fmt.Errorf("EgressNetworkPolicy in global network namespace (%s) is not allowed (%s); ignoring firewall rules", ns, policy.Name))
		return
	}

	firewall := []firewallItem{}
	for _, rule := range policy.Spec.Egress {
		if len(rule.To.CIDRSelector) > 0 {
			selector := rule.To.CIDRSelector
			if selector == "0.0.0.0/32" {
				// ovscontroller.go already logs a warning about this
				selector = "0.0.0.0/0"
			}
			_, cidr, err := net.ParseCIDR(selector)
			if err != nil {
				// should have been caught by validation
				utilruntime.HandleError(fmt.Errorf("Illegal CIDR value %q in EgressNetworkPolicy rule for policy: %v", rule.To.CIDRSelector, policy.UID))
				continue
			}
			firewall = append(firewall, firewallItem{rule.Type, cidr})
		} else if len(rule.To.DNSName) > 0 {
			cidrs := proxy.egressDNS.GetNetCIDRs(policy, rule.To.DNSName)
			for _, cidr := range cidrs {
				firewall = append(firewall, firewallItem{rule.Type, &cidr})
			}
		} else {
			// Should have been caught by validation
			utilruntime.HandleError(fmt.Errorf("Invalid EgressNetworkPolicy rule: %v for policy: %v", rule, policy.UID))
		}
	}

	// Add/Update/Delete firewall rules for the namespace
	if len(firewall) > 0 {
		if _, ok := proxy.firewall[ns]; !ok {
			item := &proxyFirewallItem{}
			item.namespaceFirewalls = make(map[ktypes.UID][]firewallItem)
			item.activePolicy = nil
			proxy.firewall[ns] = item
		}
		proxy.firewall[ns].namespaceFirewalls[policy.UID] = firewall
	} else if _, ok := proxy.firewall[ns]; ok {
		delete(proxy.firewall[ns].namespaceFirewalls, policy.UID)
		if len(proxy.firewall[ns].namespaceFirewalls) == 0 {
			delete(proxy.firewall, ns)
		}
	}

	// Set active policy for the namespace
	if ref, ok := proxy.firewall[ns]; ok {
		if len(ref.namespaceFirewalls) == 1 {
			for uid := range ref.namespaceFirewalls {
				ref.activePolicy = &uid
				klog.Infof("Applied firewall egress network policy: %q to namespace: %q", uid, ns)
			}
		} else {
			ref.activePolicy = nil
			// We only allow one policy per namespace otherwise it's hard to determine which policy to apply first
			utilruntime.HandleError(fmt.Errorf("Found multiple egress policies, dropping all firewall rules for namespace: %q", ns))
		}
	}

	// Update endpoints
	for _, pep := range proxy.allEndpoints {
		if pep.endpoints.Namespace != policy.Namespace {
			continue
		}

		wasBlocked := pep.blocked
		pep.blocked = proxy.endpointsBlocked(pep.endpoints)
		switch {
		case wasBlocked && !pep.blocked:
			proxy.baseProxy.OnEndpointsAdd(pep.endpoints)
		case !wasBlocked && pep.blocked:
			proxy.baseProxy.OnEndpointsDelete(pep.endpoints)
		}
	}
}

func (proxy *OsdnProxy) firewallBlocksIP(namespace string, ip net.IP) bool {
	if ref, ok := proxy.firewall[namespace]; ok {
		if ref.activePolicy == nil {
			// Block all connections if active policy is not set
			return true
		}

		for _, item := range ref.namespaceFirewalls[*ref.activePolicy] {
			if item.net.Contains(ip) {
				return item.ruleType == networkv1.EgressNetworkPolicyRuleDeny
			}
		}
	}
	return false
}

func (proxy *OsdnProxy) endpointsBlocked(ep *corev1.Endpoints) bool {
	for _, ss := range ep.Subsets {
		for _, addr := range ss.Addresses {
			IP := net.ParseIP(addr.IP)
			if _, contains := common.ClusterNetworkListContains(proxy.networkInfo.ClusterNetworks, IP); !contains && !proxy.networkInfo.ServiceNetwork.Contains(IP) {
				if proxy.firewallBlocksIP(ep.Namespace, IP) {
					klog.Warningf("Service '%s' in namespace '%s' has an Endpoint pointing to firewalled destination (%s)", ep.Name, ep.Namespace, addr.IP)
					return true
				}
			}
		}
	}

	return false
}

func (proxy *OsdnProxy) OnEndpointsAdd(ep *corev1.Endpoints) {
	proxy.lock.Lock()
	defer proxy.lock.Unlock()

	pep := &proxyEndpoints{ep, proxy.endpointsBlocked(ep)}
	proxy.allEndpoints[ep.UID] = pep
	if !pep.blocked {
		proxy.baseProxy.OnEndpointsAdd(ep)
	}
}

func (proxy *OsdnProxy) OnEndpointsUpdate(old, ep *corev1.Endpoints) {
	proxy.lock.Lock()
	defer proxy.lock.Unlock()

	pep := proxy.allEndpoints[ep.UID]
	if pep == nil {
		klog.Warningf("Got OnEndpointsUpdate for unknown Endpoints %#v", ep)
		pep = &proxyEndpoints{ep, true}
		proxy.allEndpoints[ep.UID] = pep
	}
	wasBlocked := pep.blocked
	pep.endpoints = ep
	pep.blocked = proxy.endpointsBlocked(ep)

	switch {
	case wasBlocked && !pep.blocked:
		proxy.baseProxy.OnEndpointsAdd(ep)
	case !wasBlocked && !pep.blocked:
		proxy.baseProxy.OnEndpointsUpdate(old, ep)
	case !wasBlocked && pep.blocked:
		proxy.baseProxy.OnEndpointsDelete(ep)
	}
}

func (proxy *OsdnProxy) OnEndpointsDelete(ep *corev1.Endpoints) {
	proxy.lock.Lock()
	defer proxy.lock.Unlock()

	pep := proxy.allEndpoints[ep.UID]
	if pep == nil {
		klog.Warningf("Got OnEndpointsDelete for unknown Endpoints %#v", ep)
		return
	}
	delete(proxy.allEndpoints, ep.UID)
	if !pep.blocked {
		proxy.baseProxy.OnEndpointsDelete(ep)
	}
}

func (proxy *OsdnProxy) OnEndpointsSynced() {
	proxy.baseProxy.OnEndpointsSynced()
}

func (proxy *OsdnProxy) OnServiceAdd(service *corev1.Service) {
	klog.V(2).Infof("sdn proxy: add svc %s/%s: %v", service.Namespace, service.Name, service)
	proxy.baseProxy.OnServiceAdd(service)
}

func (proxy *OsdnProxy) OnServiceUpdate(oldService, service *corev1.Service) {
	proxy.baseProxy.OnServiceUpdate(oldService, service)
}

func (proxy *OsdnProxy) OnServiceDelete(service *corev1.Service) {
	proxy.baseProxy.OnServiceDelete(service)
}

func (proxy *OsdnProxy) OnServiceSynced() {
	proxy.baseProxy.OnServiceSynced()
}

func (proxy *OsdnProxy) Sync() {
	proxy.baseProxy.Sync()
}

func (proxy *OsdnProxy) SyncLoop() {
	proxy.baseProxy.SyncLoop()
}

func (proxy *OsdnProxy) syncEgressDNSProxyFirewall() {
	policies, err := proxy.networkClient.NetworkV1().EgressNetworkPolicies(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Could not get EgressNetworkPolicies: %v", err))
		return
	}

	go utilwait.Forever(proxy.egressDNS.Sync, 0)

	for {
		policyUpdates := <-proxy.egressDNS.Updates
		klog.V(5).Infof("Egress dns sync: update proxy firewall for policy: %v", policyUpdates.UID)

		policy, ok := getPolicy(policyUpdates.UID, policies)
		if !ok {
			policies, err = proxy.networkClient.NetworkV1().EgressNetworkPolicies(metav1.NamespaceAll).List(metav1.ListOptions{})
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("Failed to update proxy firewall for policy: %v, Could not get EgressNetworkPolicies: %v", policyUpdates.UID, err))
				continue
			}

			policy, ok = getPolicy(policyUpdates.UID, policies)
			if !ok {
				klog.Warningf("Unable to update proxy firewall for policy: %v, policy not found", policyUpdates.UID)
				continue
			}
		}

		proxy.updateEgressNetworkPolicyLocked(policy)
	}
}

func getPolicy(policyUID ktypes.UID, policies *networkv1.EgressNetworkPolicyList) (networkv1.EgressNetworkPolicy, bool) {
	for _, p := range policies.Items {
		if p.UID == policyUID {
			return p, true
		}
	}
	return networkv1.EgressNetworkPolicy{}, false
}
