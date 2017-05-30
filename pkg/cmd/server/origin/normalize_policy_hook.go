package origin

import (
	"strings"

	"github.com/openshift/origin/pkg/authorization/api"
	authorizationinternalversion "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapiserver "k8s.io/apiserver/pkg/server"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
)

func NormalizePolicyPostStartHook(hookContext genericapiserver.PostStartHookContext) error {
	authorizationClient, err := authorizationinternalversion.NewForConfig(hookContext.LoopbackClientConfig)
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}
	if err := normalizeClusterPolicy(authorizationClient.ClusterPolicies()); err != nil {
		utilruntime.HandleError(err)
	}
	coreClient, err := coreclient.NewForConfig(hookContext.LoopbackClientConfig)
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}
	namespaces, err := coreClient.Namespaces().List(v1.ListOptions{})
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}
	for _, namespace := range namespaces.Items {
		if err := normalizePolicy(authorizationClient.Policies(namespace.Name)); err != nil {
			utilruntime.HandleError(err)
		}
	}
	return nil
}

func normalizeClusterPolicy(clusterPolicyInterface authorizationinternalversion.ClusterPolicyInterface) error {
	policies, err := clusterPolicyInterface.List(v1.ListOptions{})
	if err != nil {
		return err
	}
	for _, policy := range policies.Items {
		normalize(&policy)
		if _, err := clusterPolicyInterface.Update(&policy); err != nil {
			return err
		}
	}
	return nil
}

func normalizePolicy(policyInterface authorizationinternalversion.PolicyInterface) error {
	policies, err := policyInterface.List(v1.ListOptions{})
	if err != nil {
		return err
	}
	for _, policy := range policies.Items {
		clusterPolicy := api.ToClusterPolicy(&policy)
		normalize(clusterPolicy)
		p := api.ToPolicy(clusterPolicy)
		if _, err := policyInterface.Update(p); err != nil {
			return err
		}
	}
	return nil
}

func normalize(clusterPolicy *api.ClusterPolicy) {
	for _, role := range clusterPolicy.Roles {
		for i, rule := range role.Rules {
			rule.Verbs = toLowerSet(rule.Verbs)
			rule.Resources = toLowerSet(rule.Resources)
			rule.APIGroups = toLowerSet(sets.NewString(rule.APIGroups...)).List()
			role.Rules[i] = rule
		}
	}
}

func toLowerSet(set sets.String) sets.String {
	out := sets.NewString()
	for item := range set {
		out.Insert(strings.ToLower(item))
	}
	return out
}
