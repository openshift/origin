package bootstrappolicy

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/rbac"

	"github.com/golang/glog"
)

func addNamespaceRole(namespaceRoles map[string][]rbac.Role, namespace string, role rbac.Role) {
	if namespace != "openshift" && !strings.HasPrefix(namespace, "openshift-") {
		glog.Fatalf(`roles can only be bootstrapped into reserved "openshift" namespace or namespaces starting with "openshift-", not %q`, namespace)
	}

	existingRoles := namespaceRoles[namespace]
	for _, existingRole := range existingRoles {
		if role.Name == existingRole.Name {
			glog.Fatalf("role %q was already registered in %q", role.Name, namespace)
		}
	}

	role.Namespace = namespace
	addDefaultMetadata(&role)
	existingRoles = append(existingRoles, role)
	namespaceRoles[namespace] = existingRoles
}

func addNamespaceRoleBinding(namespaceRoleBindings map[string][]rbac.RoleBinding, namespace string, roleBinding rbac.RoleBinding) {
	if namespace != "openshift" && !strings.HasPrefix(namespace, "openshift-") {
		glog.Fatalf(`role bindings can only be bootstrapped into reserved "openshift" namespace or namespaces starting with "openshift-", not %q`, namespace)
	}

	existingRoleBindings := namespaceRoleBindings[namespace]
	for _, existingRoleBinding := range existingRoleBindings {
		if roleBinding.Name == existingRoleBinding.Name {
			glog.Fatalf("rolebinding %q was already registered in %q", roleBinding.Name, namespace)
		}
	}

	roleBinding.Namespace = namespace
	addDefaultMetadata(&roleBinding)
	existingRoleBindings = append(existingRoleBindings, roleBinding)
	namespaceRoleBindings[namespace] = existingRoleBindings
}

func buildNamespaceRolesAndBindings() (map[string][]rbac.Role, map[string][]rbac.RoleBinding) {
	// namespaceRoles is a map of namespace to slice of roles to create
	namespaceRoles := map[string][]rbac.Role{}
	// namespaceRoleBindings is a map of namespace to slice of roleBindings to create
	namespaceRoleBindings := map[string][]rbac.RoleBinding{}

	addNamespaceRole(namespaceRoles,
		DefaultOpenShiftSharedResourcesNamespace,
		rbac.Role{
			ObjectMeta: metav1.ObjectMeta{Name: OpenshiftSharedResourceViewRoleName},
			Rules: []rbac.PolicyRule{
				rbac.NewRule(read...).Groups(templateGroup, legacyTemplateGroup).Resources("templates").RuleOrDie(),
				rbac.NewRule(read...).Groups(imageGroup, legacyImageGroup).Resources("imagestreams", "imagestreamtags", "imagestreamimages").RuleOrDie(),
				// so anyone can pull from openshift/* image streams
				rbac.NewRule("get").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),
			},
		})

	addNamespaceRoleBinding(namespaceRoleBindings,
		DefaultOpenShiftSharedResourcesNamespace,
		newOriginRoleBinding(OpenshiftSharedResourceViewRoleBindingName, OpenshiftSharedResourceViewRoleName, DefaultOpenShiftSharedResourcesNamespace).Groups(AuthenticatedGroup).BindingOrDie())

	addNamespaceRole(namespaceRoles,
		DefaultOpenShiftNodeNamespace,
		rbac.Role{
			ObjectMeta: metav1.ObjectMeta{Name: NodeConfigReaderRoleName},
			Rules: []rbac.PolicyRule{
				// Allow the reader to read config maps in a given namespace with a given name.
				rbac.NewRule("get").Groups(kapiGroup).Resources("configmaps").RuleOrDie(),
			},
		})
	addNamespaceRoleBinding(namespaceRoleBindings,
		DefaultOpenShiftNodeNamespace,
		rbac.NewRoleBinding(NodeConfigReaderRoleName, DefaultOpenShiftNodeNamespace).Groups(NodesGroup).BindingOrDie())

	return namespaceRoles, namespaceRoleBindings
}

// NamespaceRoles returns a map of namespace to slice of roles to create
func NamespaceRoles() map[string][]rbac.Role {
	namespaceRoles, _ := buildNamespaceRolesAndBindings()
	return namespaceRoles
}

// NamespaceRoleBindings returns a map of namespace to slice of role bindings to create
func NamespaceRoleBindings() map[string][]rbac.RoleBinding {
	_, namespaceRoleBindings := buildNamespaceRolesAndBindings()
	return namespaceRoleBindings
}
