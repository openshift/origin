package openshift

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/rbac"
)

// Roles
const (
	ClusterServiceBrokerAdminRoleName = "system:openshift:clusterservicebroker-client"
)

// GetServiceCatalogClusterRoles returns requried cluster roles to bootstrap service catalog
func GetServiceCatalogClusterRoles() []rbac.ClusterRole {
	return []rbac.ClusterRole{
		{
			ObjectMeta: v1.ObjectMeta{
				Name:   "system:openshift:service-catalog:aggregate-to-admin",
				Labels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-admin": "true"},
			},
			Rules: []rbac.PolicyRule{
				rbac.NewRule("create", "update", "delete", "get", "list", "watch", "patch").Groups("servicecatalog.k8s.io").Resources("serviceinstances", "servicebindings").RuleOrDie(),
				rbac.NewRule("create", "update", "delete", "get", "list", "watch").Groups("settings.k8s.io").Resources("podpresets").RuleOrDie(),
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:   "system:openshift:service-catalog:aggregate-to-edit",
				Labels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-edit": "true"},
			},
			Rules: []rbac.PolicyRule{
				rbac.NewRule("create", "update", "delete", "get", "list", "watch", "patch").Groups("servicecatalog.k8s.io").Resources("serviceinstances", "servicebindings").RuleOrDie(),
				rbac.NewRule("create", "update", "delete", "get", "list", "watch").Groups("settings.k8s.io").Resources("podpresets").RuleOrDie(),
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name:   "system:openshift:service-catalog:aggregate-to-view",
				Labels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-view": "true"},
			},
			Rules: []rbac.PolicyRule{
				rbac.NewRule("get", "list", "watch").Groups("servicecatalog.k8s.io").Resources("serviceinstances", "servicebindings").RuleOrDie(),
			},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name: ClusterServiceBrokerAdminRoleName,
			},
			Rules: []rbac.PolicyRule{
				rbac.NewRule("create", "update", "delete", "get", "list", "watch", "patch").Groups("servicecatalog.k8s.io").Resources("clusterservicebrokers").RuleOrDie(),
			},
		},
	}
}
