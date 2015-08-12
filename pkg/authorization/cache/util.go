package cache

import (
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// PolicyToSelectableFields returns fields from a Policy object that support querying
func PolicyToSelectableFields(policy *authorizationapi.Policy) labels.Set {
	return labels.Set{
		"metadata.name": policy.Name,
	}
}

// PolicyBindingToSelectableFields returns fields from a Policy object that support querying
func PolicyBindingToSelectableFields(policyBinding *authorizationapi.PolicyBinding) labels.Set {
	return labels.Set{
		"metadata.name": policyBinding.Name,
	}
}

// ClusterPolicyToSelectableFields returns fields from a Policy object that support querying
func ClusterPolicyToSelectableFields(clusterPolicy *authorizationapi.ClusterPolicy) labels.Set {
	return labels.Set{
		"metadata.name": clusterPolicy.Name,
	}
}

// ClusterPolicyBindingToSelectableFields returns fields from a Policy object that support querying
func ClusterPolicyBindingToSelectableFields(clusterPolicyBinding *authorizationapi.ClusterPolicyBinding) labels.Set {
	return labels.Set{
		"metadata.name": clusterPolicyBinding.Name,
	}
}
