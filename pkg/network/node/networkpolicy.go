// +build linux

package node

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/networking"

	networkapi "github.com/openshift/api/network/v1"
	"github.com/openshift/origin/pkg/network"
	"github.com/openshift/origin/pkg/network/common"
	"github.com/openshift/origin/pkg/util/ovs"
)

type networkPolicyPlugin struct {
	node  *OsdnNode
	vnids *nodeVNIDMap

	lock        sync.Mutex
	namespaces  map[uint32]*npNamespace
	kNamespaces map[string]kapi.Namespace
	pods        map[ktypes.UID]kapi.Pod
}

// npNamespace tracks NetworkPolicy-related data for a Namespace
type npNamespace struct {
	name  string
	vnid  uint32
	inUse bool

	policies map[ktypes.UID]*npPolicy
}

// npPolicy is a parsed version of a single NetworkPolicy object
type npPolicy struct {
	policy            networking.NetworkPolicy
	watchesNamespaces bool
	watchesPods       bool

	flows       []string
	selectedIPs []string
}

type refreshForType string

const (
	refreshForPods       refreshForType = "pods"
	refreshForNamespaces refreshForType = "namespaces"
)

func NewNetworkPolicyPlugin() osdnPolicy {
	return &networkPolicyPlugin{
		namespaces:  make(map[uint32]*npNamespace),
		kNamespaces: make(map[string]kapi.Namespace),
		pods:        make(map[ktypes.UID]kapi.Pod),
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
	otx.AddFlow("table=80, priority=1, actions=output:NXM_NX_REG2[]")
	if err := otx.Commit(); err != nil {
		return err
	}

	if err := np.initNamespaces(); err != nil {
		return err
	}

	np.watchNamespaces()
	np.watchPods()
	np.watchNetworkPolicies()
	go utilwait.Forever(np.dumpDebugFlows, 5*time.Minute)
	return nil
}

func (np *networkPolicyPlugin) dumpDebugFlows() {
	np.lock.Lock()
	defer np.lock.Unlock()

	flows, _ := np.node.oc.ovs.DumpFlows("table=80")
	glog.Infof("DUMP: %d flows", len(flows))
	for _, flow := range flows {
		parsed, err := ovs.ParseFlow(ovs.ParseForDump, flow)
		if err != nil {
			glog.Errorf("DUMP: parse error %v", err)
			continue
		}
		n_packets, found := parsed.FindField("n_packets")
		if !found || n_packets.Value == "0" {
			continue
		}
		if parsed.Priority == 1 {
			glog.Infof("DUMP: %s packets hit fallback rule!", n_packets.Value)
			continue
		}

		reg0hex, found := parsed.FindField("reg0")
		if !found {
			continue
		}
		reg1hex, found := parsed.FindField("reg1")
		if !found {
			continue
		}
		reg0, err := strconv.ParseUint(reg0hex.Value, 0, 0)
		if err != nil {
			glog.Errorf("DUMP: parse error %v", err)
			continue
		}
		reg1, err := strconv.ParseUint(reg1hex.Value, 0, 0)
		if err != nil {
			glog.Errorf("DUMP: parse error %v", err)
			continue
		}
		if reg0 == reg1 {
			continue
		}
		ns0 := np.namespaces[uint32(reg0)]
		ns1 := np.namespaces[uint32(reg1)]
		if ns0 == nil && ns1 == nil {
			glog.Infof("DUMP: %d -> %d", reg0, reg1)
		} else if ns0 == nil {
			glog.Infof("DUMP: %d -> %s", reg0, ns1.name)
		} else if ns1 == nil {
			glog.Infof("DUMP: %s -> %d", ns0.name, reg1)
		} else {
			glog.Infof("DUMP: %s -> %s", ns0.name, ns1.name)
		}
	}
}

func (np *networkPolicyPlugin) initNamespaces() error {
	np.lock.Lock()
	defer np.lock.Unlock()

	namespaces, err := np.node.kClient.Core().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, ns := range namespaces.Items {
		np.kNamespaces[ns.Name] = ns

		if vnid, err := np.vnids.WaitAndGetVNID(ns.Name); err == nil {
			np.namespaces[vnid] = &npNamespace{
				name:     ns.Name,
				vnid:     vnid,
				inUse:    false,
				policies: make(map[ktypes.UID]*npPolicy),
			}
		}
	}

	policies, err := np.node.kClient.Networking().NetworkPolicies(kapi.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		if kapierrs.IsForbidden(err) {
			utilruntime.HandleError(fmt.Errorf("unable to query NetworkPolicies (%v) - please ensure your nodes have access to view NetworkPolicy", err))
		}
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

func (np *networkPolicyPlugin) AddNetNamespace(netns *networkapi.NetNamespace) {
	np.lock.Lock()
	defer np.lock.Unlock()

	if _, exists := np.namespaces[netns.NetID]; exists {
		glog.Warningf("Got AddNetNamespace for already-existing namespace %s (%d)", netns.NetName, netns.NetID)
		return
	}

	np.namespaces[netns.NetID] = &npNamespace{
		name:     netns.NetName,
		vnid:     netns.NetID,
		inUse:    false,
		policies: make(map[ktypes.UID]*npPolicy),
	}
	np.syncNamespace(np.namespaces[netns.NetID])
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

	delete(np.namespaces, netns.NetID)
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
	glog.Infof("syncNamespace %d", npns.vnid)
	otx := np.node.oc.NewTransaction()
	for otherVNID := range np.namespaces {
		otx.AddFlow("table=80, priority=150, reg0=%d, reg1=%d, actions=output:NXM_NX_REG2[]", otherVNID, npns.vnid)
		otx.AddFlow("table=80, priority=150, reg0=%d, reg1=%d, actions=output:NXM_NX_REG2[]", npns.vnid, otherVNID)
	}
	if err := otx.Commit(); err != nil {
		utilruntime.HandleError(fmt.Errorf("Error syncing OVS flows for VNID: %v", err))
	} else {
		glog.Infof("syncedNamespace %d", npns.vnid)
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
	//np.syncNamespace(npns)
}

func (np *networkPolicyPlugin) SyncVNIDRules() {
}

func (np *networkPolicyPlugin) selectPodsFromNamespaces(nsLabelSel, podLabelSel *metav1.LabelSelector) []string {
	namespaces := make(map[string]uint32)
	var peerFlows []string

	nsSel, err := metav1.LabelSelectorAsSelector(nsLabelSel)
	if err != nil {
		// Shouldn't happen
		utilruntime.HandleError(fmt.Errorf("ValidateNetworkPolicy() failure! Invalid NamespaceSelector: %v", err))
		return nil
	}

	podSel, err := metav1.LabelSelectorAsSelector(podLabelSel)
	if err != nil {
		// Shouldn't happen
		utilruntime.HandleError(fmt.Errorf("ValidateNetworkPolicy() failure! Invalid PodSelector: %v", err))
		return nil
	}

	for vnid, ns := range np.namespaces {
		if kns, exists := np.kNamespaces[ns.name]; exists {
			if nsSel.Matches(labels.Set(kns.Labels)) {
				namespaces[ns.name] = vnid
			}
		}
	}
	for _, pod := range np.pods {
		vnid, exists := namespaces[pod.Namespace]
		if exists && podSel.Matches(labels.Set(pod.Labels)) {
			peerFlows = append(peerFlows, fmt.Sprintf("reg0=%d, ip, nw_src=%s, ", vnid, pod.Status.PodIP))
		}

	}

	return peerFlows
}

func (np *networkPolicyPlugin) selectNamespaces(lsel *metav1.LabelSelector) []string {
	var vnids []string
	sel, err := metav1.LabelSelectorAsSelector(lsel)
	if err != nil {
		// Shouldn't happen
		utilruntime.HandleError(fmt.Errorf("ValidateNetworkPolicy() failure! Invalid NamespaceSelector: %v", err))
		return vnids
	}
	for vnid, ns := range np.namespaces {
		if kns, exists := np.kNamespaces[ns.name]; exists {
			if sel.Matches(labels.Set(kns.Labels)) {
				vnids = append(vnids, fmt.Sprintf("reg0=%d, ", vnid))
			}
		}
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
		// The rest of this file assumes that all policies affect ingress: a policy that
		// only affects egress is, for our purposes, equivalent to one that affects
		// ingress but does not select any pods.
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
				// FIXME: validation should catch this
				return nil, fmt.Errorf("policy specifies unrecognized protocol %q", *port.Protocol)
			}
			var portNum int
			if port.Port == nil {
				// FIXME: implement this?
				return nil, fmt.Errorf("port fields with no port value are not implemented")
			} else if port.Port.Type != intstr.Int {
				// FIXME: implement this?
				return nil, fmt.Errorf("named port values (%q) are not implemented", port.Port.StrVal)
			} else {
				portNum = int(port.Port.IntVal)
				if portNum < 0 || portNum > 0xFFFF {
					// FIXME: validation should catch this
					return nil, fmt.Errorf("port value out of bounds %q", port.Port.IntVal)
				}
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
					peerFlows = append(peerFlows, np.selectNamespaces(peer.NamespaceSelector)...)
				}
			} else {
				npp.watchesNamespaces = true
				npp.watchesPods = true
				peerFlows = append(peerFlows, np.selectPodsFromNamespaces(peer.NamespaceSelector, peer.PodSelector)...)
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
				//np.syncNamespace(npns)
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
		delete(npns.policies, policy.UID)
		if npns.inUse {
			//np.syncNamespace(npns)
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

	np.kNamespaces[ns.Name] = *ns
	np.refreshNetworkPolicies(refreshForNamespaces)
}

func (np *networkPolicyPlugin) handleDeleteNamespace(obj interface{}) {
	ns := obj.(*kapi.Namespace)
	glog.V(5).Infof("Watch %s event for Namespace %q", watch.Deleted, ns.Name)

	np.lock.Lock()
	defer np.lock.Unlock()

	delete(np.kNamespaces, ns.Name)
	np.refreshNetworkPolicies(refreshForNamespaces)
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
			//np.syncNamespace(npns)
		}
	}
}

func getPodFullName(pod *kapi.Pod) string {
	return fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
}
