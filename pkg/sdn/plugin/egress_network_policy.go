package plugin

import (
	"fmt"

	"github.com/golang/glog"

	osapi "github.com/openshift/origin/pkg/sdn/apis/network"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

func (plugin *OsdnNode) SetupEgressNetworkPolicy() error {
	policies, err := plugin.osClient.EgressNetworkPolicies(metav1.NamespaceAll).List(metav1.ListOptions{})
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
	go utilwait.Forever(plugin.watchEgressNetworkPolicies, 0)
	return nil
}

func (plugin *OsdnNode) watchEgressNetworkPolicies() {
	RunEventQueue(plugin.osClient, EgressNetworkPolicies, func(delta cache.Delta) error {
		policy := delta.Object.(*osapi.EgressNetworkPolicy)

		vnid, err := plugin.policy.GetVNID(policy.Namespace)
		if err != nil {
			return fmt.Errorf("could not find netid for namespace %q: %v", policy.Namespace, err)
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

		if delta.Type != cache.Deleted && len(policy.Spec.Egress) > 0 {
			policies = append(policies, *policy)
			plugin.egressDNS.Add(*policy)
		}
		plugin.egressPolicies[vnid] = policies

		plugin.updateEgressNetworkPolicyRules(vnid)
		return nil
	})
}

func (plugin *OsdnNode) UpdateEgressNetworkPolicyVNID(namespace string, oldVnid, newVnid uint32) {
	var policy *osapi.EgressNetworkPolicy

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
		policyUpdates := <-plugin.egressDNS.updates
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
