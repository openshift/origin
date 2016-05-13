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

	// this seems really strange, but semantic equality checks most of what we want, but nil == {}
	// this is ok for the restof the fields, but not APIGroups
	if kapi.Semantic.Equalities.DeepEqual(oldAllowAllPolicyRule, *obj) && obj.APIGroups == nil {
		obj.APIGroups = []string{internal.APIGroupAll}
	}
}

func addDefaultingFuncs(scheme *runtime.Scheme) {
	err := scheme.AddDefaultingFuncs(
		SetDefaults_PolicyRule,
	)
	if err != nil {
		panic(err)
	}
}
