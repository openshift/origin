// +build linux

package node

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/networking"
	"k8s.io/kubernetes/pkg/util/async"

	networkapi "github.com/openshift/api/network/v1"
	"github.com/openshift/origin/pkg/network"
	"github.com/openshift/origin/pkg/network/common"
)

type networkPolicyPlugin struct {
	node   *OsdnNode
	vnids  *nodeVNIDMap
	runner *async.BoundedFrequencyRunner

	lock sync.Mutex
	// namespacesByName includes every Namespace, including ones that we haven't seen
	// a NetNamespace for, and is only used in the informer-related methods.
	namespacesByName map[string]*npNamespace
	// namespaces includes only the namespaces that we have a VNID for, and is used
	// for all flow-generating methods
	namespaces map[uint32]*npNamespace
	// nsMatchCache caches matches for namespaceSelectors; see selectNamespaceInternal
	nsMatchCache map[string]*npCacheEntry

	pods map[ktypes.UID]kapi.Pod
}

// npNamespace tracks NetworkPolicy-related data for a Namespace
type npNamespace struct {
	name  string
	vnid  uint32
	inUse bool
	dirty bool

	labels   map[string]string
	policies map[ktypes.UID]*npPolicy

	gotNamespace    bool
	gotNetNamespace bool
}

// npPolicy is a parsed version of a single NetworkPolicy object
type npPolicy struct {
	policy            networking.NetworkPolicy
	watchesNamespaces bool
	watchesPods       bool

	flows       []string
	selectedIPs []string
}

// npCacheEntry caches information about matches for a LabelSelector
type npCacheEntry struct {
	selector labels.Selector
	matches  map[string]uint32
}

type refreshForType string

const (
	refreshForPods       refreshForType = "pods"
	refreshForNamespaces refreshForType = "namespaces"
)

func NewNetworkPolicyPlugin() osdnPolicy {
	return &networkPolicyPlugin{
		namespaces:       make(map[uint32]*npNamespace),
		namespacesByName: make(map[string]*npNamespace),
		pods:             make(map[ktypes.UID]kapi.Pod),

		nsMatchCache: make(map[string]*npCacheEntry),
	}
}

func (np *networkPolicyPlugin) Name() string {
	return network.NetworkPolicyPluginName
}

func (np *networkPolicyPlugin) SupportsVNIDs() bool {
	return true
}

func (np *networkPolicyPlugin) Start(node *OsdnNode) error {
	np.node = node
	np.vnids = newNodeVNIDMap(np, node.networkClient)
	if err := np.vnids.Start(node.networkInformers); err != nil {
		return err
	}

	otx := node.oc.NewTransaction()
	for _, cn := range np.node.networkInfo.ClusterNetworks {
		otx.AddFlow("table=21, priority=200, ip, nw_dst=%s, actions=ct(commit,table=30)", cn.ClusterCIDR.String())
	}
	otx.AddFlow("table=80, priority=200, ip, ct_state=+rpl, actions=output:NXM_NX_REG2[]")
	if err := otx.Commit(); err != nil {
		return err
	}

	// Rate-limit calls to np.syncFlows to 1-per-second after the 2nd call within 1
	// second. The maxInterval (time.Hour) is irrelevant here because we always call
	// np.runner.Run() if there is syncing to be done.
	np.runner = async.NewBoundedFrequencyRunner("NetworkPolicy", np.syncFlows, time.Second, time.Hour, 2)
	go np.runner.Loop(utilwait.NeverStop)

	if err := np.initNamespaces(); err != nil {
		return err
	}

	np.watchNamespaces()
	np.watchPods()
	np.watchNetworkPolicies()
	return nil
}

func (np *networkPolicyPlugin) initNamespaces() error {
	np.lock.Lock()
	defer np.lock.Unlock()

	inUseVNIDs := np.node.oc.FindPolicyVNIDs()

	namespaces, err := np.node.kClient.Core().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, ns := range namespaces.Items {
		npns := newNPNamespace(ns.Name)
		npns.labels = ns.Labels
		npns.gotNamespace = true
		np.namespacesByName[ns.Name] = npns

		if vnid, err := np.vnids.WaitAndGetVNID(ns.Name); err == nil {
			npns.vnid = vnid
			npns.inUse = inUseVNIDs.Has(int(vnid))
			npns.gotNetNamespace = true
			np.namespaces[vnid] = npns
		}
	}

	policies, err := np.node.kClient.Networking().NetworkPolicies(kapi.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, policy := range policies.Items {
		vnid, err := np.vnids.WaitAndGetVNID(policy.Namespace)
		if err != nil {
			continue
		}
		npns := np.namespaces[vnid]
		np.updateNetworkPolicy(npns, &policy)
	}

	return nil
}

func newNPNamespace(name string) *npNamespace {
	return &npNamespace{
		name:     name,
		policies: make(map[ktypes.UID]*npPolicy),
	}
}

func (np *networkPolicyPlugin) AddNetNamespace(netns *networkapi.NetNamespace) {
	np.lock.Lock()
	defer np.lock.Unlock()

	npns := np.namespacesByName[netns.NetName]
	if npns == nil {
		npns = newNPNamespace(netns.NetName)
		np.namespacesByName[netns.NetName] = npns
	}

	npns.vnid = netns.NetID
	npns.inUse = false
	np.namespaces[netns.NetID] = npns

	npns.gotNetNamespace = true
	if npns.gotNamespace {
		np.updateMatchCache(npns)
		np.refreshNetworkPolicies(refreshForNamespaces)
	}
}

func (np *networkPolicyPlugin) UpdateNetNamespace(netns *networkapi.NetNamespace, oldNetID uint32) {
	if netns.NetID != oldNetID {
		glog.Warningf("Got VNID change for namespace %s while using %s plugin", netns.NetName, network.NetworkPolicyPluginName)
	}

	np.node.podManager.UpdateLocalMulticastRules(netns.NetID)
}

func (np *networkPolicyPlugin) DeleteNetNamespace(netns *networkapi.NetNamespace) {
	np.lock.Lock()
	defer np.lock.Unlock()

	npns, exists := np.namespaces[netns.NetID]
	if !exists {
		return
	}

	if npns.inUse {
		npns.inUse = false
		// We call syncNamespaceFlows() not syncNamespace() because it
		// needs to happen before we forget about the namespace.
		np.syncNamespaceFlows(npns)
	}
	delete(np.namespaces, netns.NetID)
	npns.gotNetNamespace = false

	// We don't need to call refreshNetworkPolicies here; if the VNID doesn't get
	// reused then the stale flows won't hurt anything, and if it does get reused then
	// things will be cleaned up then. However, we do have to clean up the cache.
	np.updateMatchCache(npns)
}

func (np *networkPolicyPlugin) GetVNID(namespace string) (uint32, error) {
	return np.vnids.WaitAndGetVNID(namespace)
}

func (np *networkPolicyPlugin) GetNamespaces(vnid uint32) []string {
	return np.vnids.GetNamespaces(vnid)
}

func (np *networkPolicyPlugin) GetMulticastEnabled(vnid uint32) bool {
	return np.vnids.GetMulticastEnabled(vnid)
}

func (np *networkPolicyPlugin) syncNamespace(npns *npNamespace) {
	if !npns.dirty {
		npns.dirty = true
		np.runner.Run()
	}
}

func (np *networkPolicyPlugin) syncFlows() {
	np.lock.Lock()
	defer np.lock.Unlock()

	for _, npns := range np.namespaces {
		if npns.dirty {
			np.syncNamespaceFlows(npns)
			npns.dirty = false
		}
	}
}

func (np *networkPolicyPlugin) syncNamespaceFlows(npns *npNamespace) {
	glog.V(5).Infof("syncNamespace %d", npns.vnid)
	otx := np.node.oc.NewTransaction()
	otx.DeleteFlows("table=80, reg1=%d", npns.vnid)
	if npns.inUse {
		allPodsSelected := false

		// Add "allow" rules for all traffic allowed by a NetworkPolicy
		for _, npp := range npns.policies {
			for _, flow := range npp.flows {
				otx.AddFlow("table=80, priority=150, reg1=%d, %s actions=output:NXM_NX_REG2[]", npns.vnid, flow)
			}
			if npp.selectedIPs == nil {
				allPodsSelected = true
			}
		}

		if allPodsSelected {
			// Some policy selects all pods, so all pods are "isolated" and no
			// traffic is allowed beyond what we explicitly allowed above. (And
			// the "priority=0, actions=drop" rule will filter out all remaining
			// traffic in this Namespace).
		} else {
			// No policy selects all pods, so we need an "else accept" rule to
			// allow traffic to pod IPs that aren't selected by a policy. But
			// before that we need rules to drop any remaining traffic for any pod
			// IP that *is* selected by a policy.
			selectedIPs := sets.NewString()
			for _, npp := range npns.policies {
				for _, ip := range npp.selectedIPs {
					if !selectedIPs.Has(ip) {
						selectedIPs.Insert(ip)
						otx.AddFlow("table=80, priority=100, reg1=%d, ip, nw_dst=%s, actions=drop", npns.vnid, ip)
					}
				}
			}

			otx.AddFlow("table=80, priority=50, reg1=%d, actions=output:NXM_NX_REG2[]", npns.vnid)
		}
	}
	if err := otx.Commit(); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error syncing OVS flows for VNID: %v", err))
	}
}

func (np *networkPolicyPlugin) EnsureVNIDRules(vnid uint32) {
	np.lock.Lock()
	defer np.lock.Unlock()

	npns, exists := np.namespaces[vnid]
	if !exists || npns.inUse {
		return
	}

	npns.inUse = true
	np.syncNamespace(npns)
}

func (np *networkPolicyPlugin) SyncVNIDRules() {
	np.lock.Lock()
	defer np.lock.Unlock()

	unused := np.node.oc.FindUnusedVNIDs()
	glog.Infof("SyncVNIDRules: %d unused VNIDs", len(unused))

	for _, vnid := range unused {
		npns, exists := np.namespaces[uint32(vnid)]
		if exists {
			npns.inUse = false
			np.syncNamespace(npns)
		}
	}
}

// Match namespaces against a selector, using a cache so that, eg, when a new Namespace is
// added, we only figure out if it matches "name: default" once, rather than recomputing
// the set of namespaces that match that selector for every single "allow-from-default"
// policy in the cluster.
//
// Yes, if a selector matches against multiple labels then the order they appear in
// cacheKey here is non-deterministic, but that just means that, eg, we might compute the
// results twice rather than just once, and twice is still better than 10,000 times.
func (np *networkPolicyPlugin) selectNamespacesInternal(selector labels.Selector) map[string]uint32 {
	cacheKey := selector.String()
	match := np.nsMatchCache[cacheKey]
	if match == nil {
		match = &npCacheEntry{selector: selector, matches: make(map[string]uint32)}
		for vnid, npns := range np.namespaces {
			if npns.gotNamespace && selector.Matches(labels.Set(npns.labels)) {
				match.matches[npns.name] = vnid
			}
		}
		np.nsMatchCache[cacheKey] = match
	}
	return match.matches
}

func (np *networkPolicyPlugin) updateMatchCache(npns *npNamespace) {
	for _, match := range np.nsMatchCache {
		if npns.gotNamespace && npns.gotNetNamespace && match.selector.Matches(labels.Set(npns.labels)) {
			match.matches[npns.name] = npns.vnid
		} else {
			delete(match.matches, npns.name)
		}
	}
}

func (np *networkPolicyPlugin) flushMatchCache(lsel *metav1.LabelSelector) {
	selector, err := metav1.LabelSelectorAsSelector(lsel)
	if err != nil {
		// Shouldn't happen
		utilruntime.HandleError(fmt.Errorf("ValidateNetworkPolicy() failure! Invalid NamespaceSelector: %v", err))
		return
	}
	delete(np.nsMatchCache, selector.String())
}

func (np *networkPolicyPlugin) selectNamespaces(lsel *metav1.LabelSelector) []uint32 {
	vnids := []uint32{}
	sel, err := metav1.LabelSelectorAsSelector(lsel)
	if err != nil {
		// Shouldn't happen
		utilruntime.HandleError(fmt.Errorf("ValidateNetworkPolicy() failure! Invalid NamespaceSelector: %v", err))
		return vnids
	}

	namespaces := np.selectNamespacesInternal(sel)
	for _, vnid := range namespaces {
		vnids = append(vnids, vnid)
	}
	return vnids
}

func (np *networkPolicyPlugin) selectPods(npns *npNamespace, lsel *metav1.LabelSelector) []string {
	ips := []string{}
	sel, err := metav1.LabelSelectorAsSelector(lsel)
	if err != nil {
		// Shouldn't happen
		utilruntime.HandleError(fmt.Errorf("ValidateNetworkPolicy() failure! Invalid PodSelector: %v", err))
		return ips
	}
	for _, pod := range np.pods {
		if (npns.name == pod.Namespace) && sel.Matches(labels.Set(pod.Labels)) {
			ips = append(ips, pod.Status.PodIP)
		}
	}
	return ips
}

func (np *networkPolicyPlugin) parseNetworkPolicy(npns *npNamespace, policy *networking.NetworkPolicy) (*npPolicy, error) {
	npp := &npPolicy{policy: *policy}

	affectsIngress := false
	for _, ptype := range policy.Spec.PolicyTypes {
		if ptype == networking.PolicyTypeIngress {
			affectsIngress = true
		}
	}
	if !affectsIngress {
		// The rest of this function assumes that all policies affect ingress: a
		// policy that only affects egress is, for our purposes, equivalent to one
		// that affects ingress but does not select any pods.
		npp.selectedIPs = []string{""}
		return npp, nil
	}

	var destFlows []string
	if len(policy.Spec.PodSelector.MatchLabels) > 0 || len(policy.Spec.PodSelector.MatchExpressions) > 0 {
		npp.watchesPods = true
		npp.selectedIPs = np.selectPods(npns, &policy.Spec.PodSelector)
		for _, ip := range npp.selectedIPs {
			destFlows = append(destFlows, fmt.Sprintf("ip, nw_dst=%s, ", ip))
		}
	} else {
		npp.selectedIPs = nil
		destFlows = []string{""}
	}

	for _, rule := range policy.Spec.Ingress {
		var portFlows, peerFlows []string
		if len(rule.Ports) == 0 {
			portFlows = []string{""}
		}
		for _, port := range rule.Ports {
			var protocol string
			if port.Protocol == nil {
				protocol = "tcp"
			} else if *port.Protocol == kapi.ProtocolTCP || *port.Protocol == kapi.ProtocolUDP {
				protocol = strings.ToLower(string(*port.Protocol))
			} else {
				// upstream is unlikely to add any more protocol values, but just in case...
				return nil, fmt.Errorf("policy specifies unrecognized protocol %q", *port.Protocol)
			}
			var portNum int
			if port.Port == nil {
				// FIXME: implement this?
				return nil, fmt.Errorf("port fields with no port value are not implemented")
			} else if port.Port.Type != intstr.Int {
				return nil, fmt.Errorf("named port values (%q) are not implemented", port.Port.StrVal)
			} else {
				portNum = int(port.Port.IntVal)
			}
			portFlows = append(portFlows, fmt.Sprintf("%s, tp_dst=%d, ", protocol, portNum))
		}

		if len(rule.From) == 0 {
			peerFlows = []string{""}
		}
		for _, peer := range rule.From {
			if peer.PodSelector != nil && peer.NamespaceSelector == nil {
				if len(peer.PodSelector.MatchLabels) == 0 && len(peer.PodSelector.MatchExpressions) == 0 {
					// The PodSelector is empty, meaning it selects all pods in this namespace
					peerFlows = append(peerFlows, fmt.Sprintf("reg0=%d, ", npns.vnid))
				} else {
					npp.watchesPods = true
					for _, ip := range np.selectPods(npns, peer.PodSelector) {
						peerFlows = append(peerFlows, fmt.Sprintf("reg0=%d, ip, nw_src=%s, ", npns.vnid, ip))
					}
				}
			} else if peer.NamespaceSelector != nil && peer.PodSelector == nil {
				if len(peer.NamespaceSelector.MatchLabels) == 0 && len(peer.NamespaceSelector.MatchExpressions) == 0 {
					// The NamespaceSelector is empty, meaning it selects all namespaces
					peerFlows = append(peerFlows, "")
				} else {
					npp.watchesNamespaces = true
					for _, otherVNID := range np.selectNamespaces(peer.NamespaceSelector) {
						peerFlows = append(peerFlows, fmt.Sprintf("reg0=%d, ", otherVNID))
					}
				}
			}
		}

		for _, destFlow := range destFlows {
			for _, peerFlow := range peerFlows {
				for _, portFlow := range portFlows {
					npp.flows = append(npp.flows, fmt.Sprintf("%s%s%s", destFlow, peerFlow, portFlow))
				}
			}
		}
	}

	sort.Strings(npp.flows)
	glog.V(5).Infof("Parsed NetworkPolicy: %#v", npp)
	return npp, nil
}

// Cleans up after a NetworkPolicy that is being deleted
func (np *networkPolicyPlugin) cleanupNetworkPolicy(policy *networking.NetworkPolicy) {
	for _, rule := range policy.Spec.Ingress {
		for _, peer := range rule.From {
			if peer.NamespaceSelector != nil {
				if len(peer.NamespaceSelector.MatchLabels) != 0 || len(peer.NamespaceSelector.MatchExpressions) != 0 {
					// This is overzealous; there may still be other policies
					// with the same selector. But it's simple.
					np.flushMatchCache(peer.NamespaceSelector)
				}
			}
		}
	}
}

func (np *networkPolicyPlugin) updateNetworkPolicy(npns *npNamespace, policy *networking.NetworkPolicy) bool {
	npp, err := np.parseNetworkPolicy(npns, policy)
	if err != nil {
		glog.Infof("Unsupported NetworkPolicy %s/%s (%v); treating as deny-all", policy.Namespace, policy.Name, err)
		npp = &npPolicy{policy: *policy}
	}

	oldNPP, existed := npns.policies[policy.UID]
	npns.policies[policy.UID] = npp

	changed := !existed || !reflect.DeepEqual(oldNPP.flows, npp.flows)
	if !changed {
		glog.V(5).Infof("NetworkPolicy %s/%s is unchanged", policy.Namespace, policy.Name)
	}
	return changed
}

func (np *networkPolicyPlugin) watchNetworkPolicies() {
	funcs := common.InformerFuncs(&networking.NetworkPolicy{}, np.handleAddOrUpdateNetworkPolicy, np.handleDeleteNetworkPolicy)
	np.node.kubeInformers.Networking().InternalVersion().NetworkPolicies().Informer().AddEventHandler(funcs)
}

func (np *networkPolicyPlugin) handleAddOrUpdateNetworkPolicy(obj, _ interface{}, eventType watch.EventType) {
	policy := obj.(*networking.NetworkPolicy)
	glog.V(5).Infof("Watch %s event for NetworkPolicy %s/%s", eventType, policy.Namespace, policy.Name)

	vnid, err := np.vnids.WaitAndGetVNID(policy.Namespace)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Could not find VNID for NetworkPolicy %s/%s", policy.Namespace, policy.Name))
		return
	}

	np.lock.Lock()
	defer np.lock.Unlock()

	if npns, exists := np.namespaces[vnid]; exists {
		if changed := np.updateNetworkPolicy(npns, policy); changed {
			if npns.inUse {
				np.syncNamespace(npns)
			}
		}
	}
}

func (np *networkPolicyPlugin) handleDeleteNetworkPolicy(obj interface{}) {
	policy := obj.(*networking.NetworkPolicy)
	glog.V(5).Infof("Watch %s event for NetworkPolicy %s/%s", watch.Deleted, policy.Namespace, policy.Name)

	vnid, err := np.vnids.WaitAndGetVNID(policy.Namespace)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Could not find VNID for NetworkPolicy %s/%s", policy.Namespace, policy.Name))
		return
	}

	np.lock.Lock()
	defer np.lock.Unlock()

	if npns, exists := np.namespaces[vnid]; exists {
		np.cleanupNetworkPolicy(policy)
		delete(npns.policies, policy.UID)
		if npns.inUse {
			np.syncNamespace(npns)
		}
	}
}

func (np *networkPolicyPlugin) watchPods() {
	funcs := common.InformerFuncs(&kapi.Pod{}, np.handleAddOrUpdatePod, np.handleDeletePod)
	np.node.kubeInformers.Core().InternalVersion().Pods().Informer().AddEventHandler(funcs)
}

func (np *networkPolicyPlugin) handleAddOrUpdatePod(obj, _ interface{}, eventType watch.EventType) {
	pod := obj.(*kapi.Pod)
	glog.V(5).Infof("Watch %s event for Pod %q", eventType, getPodFullName(pod))

	// Ignore pods with HostNetwork=true, SDN is not involved in this case
	if pod.Spec.SecurityContext != nil && pod.Spec.SecurityContext.HostNetwork {
		return
	}
	if pod.Status.PodIP == "" {
		glog.V(5).Infof("PodIP is not set for pod %q; ignoring", getPodFullName(pod))
		return
	}

	// We don't want to grab np.Lock for every Pod.Status change...
	// But it's safe to look up oldPod without locking here because no other
	// threads modify this map.
	oldPod, podExisted := np.pods[pod.UID]
	if podExisted && oldPod.Status.PodIP == pod.Status.PodIP && reflect.DeepEqual(oldPod.Labels, pod.Labels) {
		return
	}

	np.lock.Lock()
	defer np.lock.Unlock()

	np.pods[pod.UID] = *pod
	np.refreshNetworkPolicies(refreshForPods)
}

func (np *networkPolicyPlugin) handleDeletePod(obj interface{}) {
	pod := obj.(*kapi.Pod)
	glog.V(5).Infof("Watch %s event for Pod %q", watch.Deleted, getPodFullName(pod))

	_, podExisted := np.pods[pod.UID]
	if !podExisted {
		return
	}

	np.lock.Lock()
	defer np.lock.Unlock()

	delete(np.pods, pod.UID)
	np.refreshNetworkPolicies(refreshForPods)
}

func (np *networkPolicyPlugin) watchNamespaces() {
	funcs := common.InformerFuncs(&kapi.Namespace{}, np.handleAddOrUpdateNamespace, np.handleDeleteNamespace)
	np.node.kubeInformers.Core().InternalVersion().Namespaces().Informer().AddEventHandler(funcs)
}

func (np *networkPolicyPlugin) handleAddOrUpdateNamespace(obj, _ interface{}, eventType watch.EventType) {
	ns := obj.(*kapi.Namespace)
	glog.V(5).Infof("Watch %s event for Namespace %q", eventType, ns.Name)

	np.lock.Lock()
	defer np.lock.Unlock()

	npns := np.namespacesByName[ns.Name]
	if npns == nil {
		npns = newNPNamespace(ns.Name)
		np.namespacesByName[ns.Name] = npns
	}

	if npns.gotNamespace && reflect.DeepEqual(npns.labels, ns.Labels) {
		return
	}
	npns.labels = ns.Labels

	npns.gotNamespace = true
	if npns.gotNetNamespace {
		np.updateMatchCache(npns)
		np.refreshNetworkPolicies(refreshForNamespaces)
	}
}

func (np *networkPolicyPlugin) handleDeleteNamespace(obj interface{}) {
	ns := obj.(*kapi.Namespace)
	glog.V(5).Infof("Watch %s event for Namespace %q", watch.Deleted, ns.Name)

	np.lock.Lock()
	defer np.lock.Unlock()

	npns := np.namespacesByName[ns.Name]
	if npns == nil {
		return
	}

	delete(np.namespacesByName, ns.Name)
	npns.gotNamespace = false

	// We don't need to call refreshNetworkPolicies here; if the VNID doesn't get
	// reused then the stale flows won't hurt anything, and if it does get reused then
	// things will be cleaned up then. However, we do have to clean up the cache.
	np.updateMatchCache(npns)
}

func (np *networkPolicyPlugin) refreshNetworkPolicies(refreshFor refreshForType) {
	for _, npns := range np.namespaces {
		changed := false
		for _, npp := range npns.policies {
			if ((refreshFor == refreshForNamespaces) && npp.watchesNamespaces) ||
				((refreshFor == refreshForPods) && npp.watchesPods) {
				if np.updateNetworkPolicy(npns, &npp.policy) {
					changed = true
					break
				}
			}
		}
		if changed && npns.inUse {
			np.syncNamespace(npns)
		}
	}
}

func getPodFullName(pod *kapi.Pod) string {
	return fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
}
