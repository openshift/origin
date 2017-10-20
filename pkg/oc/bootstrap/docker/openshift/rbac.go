package openshift

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/rbac"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

// GetServiceCatalogRBACDelta returns a cluster role with the required rules to bootstrap service catalog
func GetServiceCatalogRBACDelta() []rbac.ClusterRole {
	return []rbac.ClusterRole{
		{
			ObjectMeta: v1.ObjectMeta{
				Name: bootstrappolicy.AdminRoleName,
			},
			Rules: []rbac.PolicyRule{
				rbac.NewRule("create", "update", "delete", "get", "list", "watch").Groups("servicecatalog.k8s.io").Resources("serviceinstances", "servicebindings").RuleOrDie(),
				rbac.NewRule("create", "update", "delete", "get", "list", "watch").Groups("settings.k8s.io").Resources("podpresets").RuleOrDie(),
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name: bootstrappolicy.EditRoleName,
			},
			Rules: []rbac.PolicyRule{
				rbac.NewRule("create", "update", "delete", "get", "list", "watch").Groups("servicecatalog.k8s.io").Resources("serviceinstances", "servicebindings").RuleOrDie(),
				rbac.NewRule("create", "update", "delete", "get", "list", "watch").Groups("settings.k8s.io").Resources("podpresets").RuleOrDie(),
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name: bootstrappolicy.ViewRoleName,
			},
			Rules: []rbac.PolicyRule{
				rbac.NewRule("get", "list", "watch").Groups("servicecatalog.k8s.io").Resources("serviceinstances", "servicebindings").RuleOrDie(),
			},
		},
	}
}
