package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	internal "github.com/openshift/origin/pkg/authorization/api"
)

var oldAllowAllPolicyRule = PolicyRule{APIGroups: nil, Verbs: []string{internal.VerbAll}, Resources: []string{internal.ResourceAll}}

func SetDefaults_PolicyRule(obj *PolicyRule) {
	if obj == nil {
		return
	}

	// match the old allow all rule, but only if API groups is nil (not specified in the incoming JSON)
	oldAllowAllRule := obj.APIGroups == nil &&
		// avoid calling the very expensive DeepEqual by inlining specific checks
		len(obj.Verbs) == 1 && obj.Verbs[0] == internal.VerbAll &&
		len(obj.Resources) == 1 && obj.Resources[0] == internal.ResourceAll &&
		len(obj.AttributeRestrictions.Raw) == 0 && len(obj.ResourceNames) == 0 &&
		len(obj.NonResourceURLsSlice) == 0 &&
		// semantic equalities will ignore nil vs empty for other fields as a safety
		// DO NOT REMOVE THIS CHECK unless you replace it with full equality comparisons
		kapi.Semantic.Equalities.DeepEqual(oldAllowAllPolicyRule, *obj)

	if oldAllowAllRule {
		obj.APIGroups = []string{internal.APIGroupAll}
	}

	// if no groups are found, then we assume ""
	if len(obj.Resources) > 0 && len(obj.APIGroups) == 0 {
		obj.APIGroups = []string{""}
	}
}

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return scheme.AddDefaultingFuncs(
		SetDefaults_PolicyRule,
	)
}
