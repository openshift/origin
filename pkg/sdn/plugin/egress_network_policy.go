package plugin

import (
	"fmt"

	"github.com/golang/glog"

	osapi "github.com/openshift/origin/pkg/sdn/api"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
)

func (plugin *OsdnNode) SetupEgressNetworkPolicy() error {
	policies, err := plugin.osClient.EgressNetworkPolicies(kapi.NamespaceAll).List(kapi.ListOptions{})
	if err != nil {
		return fmt.Errorf("could not get EgressNetworkPolicies: %s", err)
	}

	for _, policy := range policies.Items {
		vnid, err := plugin.vnids.GetVNID(policy.Namespace)
		if err != nil {
			glog.Warningf("Could not find netid for namespace %q: %v", policy.Namespace, err)
			continue
		}
		plugin.egressPolicies[vnid] = append(plugin.egressPolicies[vnid], &policy)
	}

	for vnid := range plugin.egressPolicies {
		err := plugin.updateEgressNetworkPolicyRules(vnid)
		if err != nil {
			return err
		}
	}

	go utilwait.Forever(plugin.watchEgressNetworkPolicies, 0)
	return nil
}

func (plugin *OsdnNode) watchEgressNetworkPolicies() {
	RunEventQueue(plugin.osClient, EgressNetworkPolicies, func(delta cache.Delta) error {
		policy := delta.Object.(*osapi.EgressNetworkPolicy)

		vnid, err := plugin.vnids.GetVNID(policy.Namespace)
		if err != nil {
			return fmt.Errorf("Could not find netid for namespace %q: %v", policy.Namespace, err)
		}

		policies := plugin.egressPolicies[vnid]
		for i, oldPolicy := range policies {
			if oldPolicy.UID == policy.UID {
				policies = append(policies[:i], policies[i+1:]...)
				break
			}
		}
		if delta.Type != cache.Deleted && len(policy.Spec.Egress) > 0 {
			policies = append(policies, policy)
		}
		plugin.egressPolicies[vnid] = policies

		err = plugin.updateEgressNetworkPolicyRules(vnid)
		if err != nil {
			return err
		}
		return nil
	})
}

func (plugin *OsdnNode) UpdateEgressNetworkPolicyVNID(namespace string, oldVnid, newVnid uint32) error {
	var policy *osapi.EgressNetworkPolicy

	policies := plugin.egressPolicies[oldVnid]
	for i, oldPolicy := range policies {
		if oldPolicy.Namespace == namespace {
			policy = oldPolicy
			plugin.egressPolicies[oldVnid] = append(policies[:i], policies[i+1:]...)
			err := plugin.updateEgressNetworkPolicyRules(oldVnid)
			if err != nil {
				return err
			}
			break
		}
	}

	if policy != nil {
		plugin.egressPolicies[newVnid] = append(plugin.egressPolicies[newVnid], policy)
		err := plugin.updateEgressNetworkPolicyRules(newVnid)
		if err != nil {
			return err
		}
	}

	return nil
}
