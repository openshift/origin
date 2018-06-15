// +build linux

package node

import (
	"fmt"

	"github.com/golang/glog"

	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/network/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

func (plugin *OsdnNode) SetupEgressNetworkPolicy() error {
	policies, err := plugin.networkClient.Network().EgressNetworkPolicies(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("could not get EgressNetworkPolicies: %s", err)
	}

	plugin.egressPoliciesLock.Lock()
	defer plugin.egressPoliciesLock.Unlock()

	for _, policy := range policies.Items {
		vnid, err := plugin.policy.GetVNID(policy.Namespace)
		if err != nil {
			glog.Warningf("Could not find netid for namespace %q: %v", policy.Namespace, err)
			continue
		}
		plugin.egressPolicies[vnid] = append(plugin.egressPolicies[vnid], policy)

		plugin.egressDNS.Add(policy)
	}

	for vnid := range plugin.egressPolicies {
		plugin.updateEgressNetworkPolicyRules(vnid)
	}

	go utilwait.Forever(plugin.syncEgressDNSPolicyRules, 0)
	plugin.watchEgressNetworkPolicies()
	return nil
}

func (plugin *OsdnNode) watchEgressNetworkPolicies() {
	funcs := common.InformerFuncs(&networkapi.EgressNetworkPolicy{}, plugin.handleAddOrUpdateEgressNetworkPolicy, plugin.handleDeleteEgressNetworkPolicy)
	plugin.networkInformers.Network().InternalVersion().EgressNetworkPolicies().Informer().AddEventHandler(funcs)
}

func (plugin *OsdnNode) handleAddOrUpdateEgressNetworkPolicy(obj, _ interface{}, eventType watch.EventType) {
	policy := obj.(*networkapi.EgressNetworkPolicy)
	glog.V(5).Infof("Watch %s event for EgressNetworkPolicy %s/%s", eventType, policy.Namespace, policy.Name)

	plugin.handleEgressNetworkPolicy(policy, eventType)
}

func (plugin *OsdnNode) handleDeleteEgressNetworkPolicy(obj interface{}) {
	policy := obj.(*networkapi.EgressNetworkPolicy)
	glog.V(5).Infof("Watch %s event for EgressNetworkPolicy %s/%s", watch.Deleted, policy.Namespace, policy.Name)

	plugin.handleEgressNetworkPolicy(policy, watch.Deleted)
}

func (plugin *OsdnNode) handleEgressNetworkPolicy(policy *networkapi.EgressNetworkPolicy, eventType watch.EventType) {
	vnid, err := plugin.policy.GetVNID(policy.Namespace)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Could not find netid for namespace %q: %v", policy.Namespace, err))
		return
	}

	plugin.egressPoliciesLock.Lock()
	defer plugin.egressPoliciesLock.Unlock()

	policies := plugin.egressPolicies[vnid]
	for i, oldPolicy := range policies {
		if oldPolicy.UID == policy.UID {
			policies = append(policies[:i], policies[i+1:]...)
			break
		}
	}
	plugin.egressDNS.Delete(*policy)

	if eventType != watch.Deleted && len(policy.Spec.Egress) > 0 {
		policies = append(policies, *policy)
		plugin.egressDNS.Add(*policy)
	}
	plugin.egressPolicies[vnid] = policies

	plugin.updateEgressNetworkPolicyRules(vnid)
}

func (plugin *OsdnNode) UpdateEgressNetworkPolicyVNID(namespace string, oldVnid, newVnid uint32) {
	var policy *networkapi.EgressNetworkPolicy

	plugin.egressPoliciesLock.Lock()
	defer plugin.egressPoliciesLock.Unlock()

	policies := plugin.egressPolicies[oldVnid]
	for i, oldPolicy := range policies {
		if oldPolicy.Namespace == namespace {
			policy = &oldPolicy
			plugin.egressPolicies[oldVnid] = append(policies[:i], policies[i+1:]...)
			plugin.updateEgressNetworkPolicyRules(oldVnid)
			break
		}
	}

	if policy != nil {
		plugin.egressPolicies[newVnid] = append(plugin.egressPolicies[newVnid], *policy)
		plugin.updateEgressNetworkPolicyRules(newVnid)
	}
}

func (plugin *OsdnNode) syncEgressDNSPolicyRules() {
	go utilwait.Forever(plugin.egressDNS.Sync, 0)

	for {
		policyUpdates := <-plugin.egressDNS.Updates
		glog.V(5).Infof("Egress dns sync: updating policy: %v", policyUpdates.UID)

		vnid, err := plugin.policy.GetVNID(policyUpdates.Namespace)
		if err != nil {
			glog.Warningf("Could not find netid for namespace %q: %v", policyUpdates.Namespace, err)
			continue
		}

		func() {
			plugin.egressPoliciesLock.Lock()
			defer plugin.egressPoliciesLock.Unlock()

			plugin.updateEgressNetworkPolicyRules(vnid)
		}()
	}
}
