package origin

import (
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	authorizerunion "k8s.io/apiserver/pkg/authorization/union"
	"k8s.io/kubernetes/pkg/auth/nodeidentifier"
	rbacinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/rbac/internalversion"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/node"
	rbacauthorizer "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"
	kbootstrappolicy "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"

	openshiftauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/authorizer/browsersafe"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
)

func NewAuthorizer(informers InformerAccess, projectRequestDenyMessage string) authorizer.Authorizer {
	messageMaker := openshiftauthorizer.NewForbiddenMessageResolver(projectRequestDenyMessage)
	rbacInformers := informers.GetInternalKubeInformers().Rbac().InternalVersion()

	scopeLimitedAuthorizer := scope.NewAuthorizer(rbacInformers.ClusterRoles().Lister(), messageMaker)

	kubeAuthorizer := rbacauthorizer.New(
		&rbacauthorizer.RoleGetter{Lister: rbacInformers.Roles().Lister()},
		&rbacauthorizer.RoleBindingLister{Lister: rbacInformers.RoleBindings().Lister()},
		&rbacauthorizer.ClusterRoleGetter{Lister: rbacInformers.ClusterRoles().Lister()},
		&rbacauthorizer.ClusterRoleBindingLister{Lister: rbacInformers.ClusterRoleBindings().Lister()},
	)

	graph := node.NewGraph()
	node.AddGraphEventHandlers(graph, informers.GetInternalKubeInformers().Core().InternalVersion().Pods(), informers.GetInternalKubeInformers().Core().InternalVersion().PersistentVolumes())
	nodeAuthorizer := node.NewAuthorizer(graph, nodeidentifier.NewDefaultNodeIdentifier(), kbootstrappolicy.NodeRules())

	openshiftAuthorizer := authorizerunion.New(
		// Wrap with an authorizer that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately.
		// Scopes are first because they will authoritatively deny and can logically be attached to anyone.
		browsersafe.NewBrowserSafeAuthorizer(scopeLimitedAuthorizer, user.AllAuthenticated),
		// authorizes system:masters to do anything, just like upstream
		authorizerfactory.NewPrivilegedGroups(user.SystemPrivilegedGroup),
		nodeAuthorizer,
		// Wrap with an authorizer that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately
		browsersafe.NewBrowserSafeAuthorizer(openshiftauthorizer.NewAuthorizer(kubeAuthorizer, messageMaker), user.AllAuthenticated),
	)

	return openshiftAuthorizer
}

func NewRuleResolver(informers rbacinformers.Interface) rbacregistryvalidation.AuthorizationRuleResolver {
	return rbacregistryvalidation.NewDefaultRuleResolver(
		&rbacauthorizer.RoleGetter{Lister: informers.Roles().Lister()},
		&rbacauthorizer.RoleBindingLister{Lister: informers.RoleBindings().Lister()},
		&rbacauthorizer.ClusterRoleGetter{Lister: informers.ClusterRoles().Lister()},
		&rbacauthorizer.ClusterRoleBindingLister{Lister: informers.ClusterRoleBindings().Lister()},
	)
}

func NewSubjectLocator(informers rbacinformers.Interface) rbacauthorizer.SubjectLocator {
	return rbacauthorizer.NewSubjectAccessEvaluator(
		&rbacauthorizer.RoleGetter{Lister: informers.Roles().Lister()},
		&rbacauthorizer.RoleBindingLister{Lister: informers.RoleBindings().Lister()},
		&rbacauthorizer.ClusterRoleGetter{Lister: informers.ClusterRoles().Lister()},
		&rbacauthorizer.ClusterRoleBindingLister{Lister: informers.ClusterRoleBindings().Lister()},
		"",
	)
}
