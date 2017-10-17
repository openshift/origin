package origin

import (
	"k8s.io/apiserver/pkg/authentication/user"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	authorizerunion "k8s.io/apiserver/pkg/authorization/union"
	"k8s.io/kubernetes/pkg/auth/nodeidentifier"
	coreinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/core/internalversion"
	rbacinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion/rbac/internalversion"
	rbaclisters "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/node"
	rbacauthorizer "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"
	kbootstrappolicy "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"

	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/authorizer/browsersafe"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
)

func NewAuthorizer(informers InformerAccess, projectRequestMessage string) (kauthorizer.Authorizer, authorizer.SubjectLocator, rbacregistryvalidation.AuthorizationRuleResolver) {
	kubeAuthorizer, ruleResolver, kubeSubjectLocator := buildKubeAuth(informers.GetInternalKubeInformers().Rbac().InternalVersion())
	authorizer, subjectLocator := newAuthorizer(
		kubeAuthorizer,
		kubeSubjectLocator,
		informers.GetInternalKubeInformers().Rbac().InternalVersion().ClusterRoles().Lister(),
		informers.GetInternalKubeInformers().Core().InternalVersion().Pods(),
		informers.GetInternalKubeInformers().Core().InternalVersion().PersistentVolumes(),
		projectRequestMessage,
	)

	return authorizer, subjectLocator, ruleResolver
}

func buildKubeAuth(r rbacinformers.Interface) (kauthorizer.Authorizer, rbacregistryvalidation.AuthorizationRuleResolver, rbacauthorizer.SubjectLocator) {
	roles := &rbacauthorizer.RoleGetter{Lister: r.Roles().Lister()}
	roleBindings := &rbacauthorizer.RoleBindingLister{Lister: r.RoleBindings().Lister()}
	clusterRoles := &rbacauthorizer.ClusterRoleGetter{Lister: r.ClusterRoles().Lister()}
	clusterRoleBindings := &rbacauthorizer.ClusterRoleBindingLister{Lister: r.ClusterRoleBindings().Lister()}
	kubeAuthorizer := rbacauthorizer.New(roles, roleBindings, clusterRoles, clusterRoleBindings)
	ruleResolver := rbacregistryvalidation.NewDefaultRuleResolver(roles, roleBindings, clusterRoles, clusterRoleBindings)
	kubeSubjectLocator := rbacauthorizer.NewSubjectAccessEvaluator(roles, roleBindings, clusterRoles, clusterRoleBindings, "")
	return kubeAuthorizer, ruleResolver, kubeSubjectLocator
}

func newAuthorizer(
	kubeAuthorizer kauthorizer.Authorizer,
	kubeSubjectLocator rbacauthorizer.SubjectLocator,
	clusterRoleGetter rbaclisters.ClusterRoleLister,
	podInformer coreinformers.PodInformer,
	pvInformer coreinformers.PersistentVolumeInformer,
	projectRequestDenyMessage string,
) (kauthorizer.Authorizer, authorizer.SubjectLocator) {
	messageMaker := authorizer.NewForbiddenMessageResolver(projectRequestDenyMessage)
	roleBasedAuthorizer := authorizer.NewAuthorizer(kubeAuthorizer, messageMaker)
	subjectLocator := authorizer.NewSubjectLocator(kubeSubjectLocator)

	scopeLimitedAuthorizer := scope.NewAuthorizer(roleBasedAuthorizer, clusterRoleGetter, messageMaker)
	// Wrap with an authorizer that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately
	browserSafeAuthorizer := browsersafe.NewBrowserSafeAuthorizer(scopeLimitedAuthorizer, user.AllAuthenticated)

	graph := node.NewGraph()
	node.AddGraphEventHandlers(graph, podInformer, pvInformer)
	nodeAuthorizer := node.NewAuthorizer(graph, nodeidentifier.NewDefaultNodeIdentifier(), kbootstrappolicy.NodeRules())

	authorizer := authorizerunion.New(
		authorizerfactory.NewPrivilegedGroups(user.SystemPrivilegedGroup), // authorizes system:masters to do anything, just like upstream
		nodeAuthorizer,
		browserSafeAuthorizer,
	)

	return authorizer, subjectLocator
}
