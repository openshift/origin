package bootstrappolicy

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"
)

var (
	// namespaceRoles is a map of namespace to slice of roles to create
	namespaceRoles = map[string][]rbac.Role{}

	// namespaceRoleBindings is a map of namespace to slice of roleBindings to create
	namespaceRoleBindings = map[string][]rbac.RoleBinding{}
)

func init() {
	namespaceRoles[DefaultOpenShiftSharedResourcesNamespace] = GetBootstrapOpenshiftRoles(DefaultOpenShiftSharedResourcesNamespace)
	namespaceRoleBindings[DefaultOpenShiftSharedResourcesNamespace] = GetBootstrapOpenshiftRoleBindings(DefaultOpenShiftSharedResourcesNamespace)
}

func GetBootstrapOpenshiftRoles(openshiftNamespace string) []rbac.Role {
	return []rbac.Role{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      OpenshiftSharedResourceViewRoleName,
				Namespace: openshiftNamespace,
			},
			Rules: []rbac.PolicyRule{
				rbac.NewRule(read...).
					Groups(templateGroup, legacyTemplateGroup).
					Resources("templates").
					RuleOrDie(),
				rbac.NewRule(read...).
					Groups(imageGroup, legacyImageGroup).
					Resources("imagestreams", "imagestreamtags", "imagestreamimages").
					RuleOrDie(),
				// so anyone can pull from openshift/* image streams
				rbac.NewRule("get").
					Groups(imageGroup, legacyImageGroup).
					Resources("imagestreams/layers").
					RuleOrDie(),
			},
		},
	}
}

// NamespaceRoles returns a map of namespace to slice of roles to create
func NamespaceRoles() map[string][]rbac.Role {
	ret := map[string][]rbac.Role{}
	for k, v := range namespaceRoles {
		ret[k] = v
	}
	for k, v := range bootstrappolicy.NamespaceRoles() {
		ret[k] = v
	}
	return ret
}

// NamespaceRoleBindings returns a map of namespace to slice of roles to create
func NamespaceRoleBindings() map[string][]rbac.RoleBinding {
	ret := map[string][]rbac.RoleBinding{}
	for k, v := range namespaceRoleBindings {
		ret[k] = v
	}
	for k, v := range bootstrappolicy.NamespaceRoleBindings() {
		ret[k] = v
	}
	return ret
}
