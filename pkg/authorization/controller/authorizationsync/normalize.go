package authorizationsync

import (
	"strings"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/apis/rbac"
)

// NormalizePolicyRules mutates the given rules and lowercases verbs, resources and API groups.
// Origin's authorizer is case-insensitive to these fields but Kubernetes RBAC is not.  Thus normalizing
// the Origin rules before persisting them as RBAC will allow authorization to continue to work.
func NormalizePolicyRules(rules []rbac.PolicyRule) {
	for i := range rules {
		rule := &rules[i]
		toLowerSlice(rule.Verbs)
		toLowerSlice(rule.APIGroups)
		rule.Resources = authorizationapi.NormalizeResources(sets.NewString(rule.Resources...)).List()
	}
}

func toLowerSlice(slice []string) {
	for i, item := range slice {
		slice[i] = strings.ToLower(item)
	}
}
