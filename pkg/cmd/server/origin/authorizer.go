package origin

import (
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	authorizerunion "k8s.io/apiserver/pkg/authorization/union"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	rbacinformers "k8s.io/client-go/informers/rbac/v1"
	"k8s.io/kubernetes/pkg/auth/nodeidentifier"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/node"
	rbacauthorizer "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"
	kbootstrappolicy "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"

	openshiftauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/authorizer/accessrestriction"
	"github.com/openshift/origin/pkg/authorization/authorizer/browsersafe"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/features"

	"github.com/golang/glog"
)

func NewAuthorizer(informers InformerAccess, projectRequestDenyMessage string) authorizer.Authorizer {
	messageMaker := openshiftauthorizer.NewForbiddenMessageResolver(projectRequestDenyMessage)
	rbacInformers := informers.GetExternalKubeInformers().Rbac().V1()

	scopeLimitedAuthorizer := scope.NewAuthorizer(rbacInformers.ClusterRoles().Lister(), messageMaker)

	// denyAuthorizer must be nil when the feature is disabled
	// thus any wrapping code like the browser safe authorizer must exist inside of the if block
	var denyAuthorizer authorizer.Authorizer
	if utilfeature.DefaultFeatureGate.Enabled(features.AccessRestrictionDenyAuthorizer) {
		glog.Warning("experimental deny authorizer is enabled - this cluster cannot be supported")
		accessRestrictionInformer := informers.GetExternalAuthorizationInformers().Authorization().V1alpha1().AccessRestrictions()
		userInformer := informers.GetUserInformers().User().V1()
		accessRestrictionAuthorizer := accessrestriction.NewAuthorizer(accessRestrictionInformer, userInformer.Users(), userInformer.Groups())
		// Wrap with an authorizer that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately.
		denyAuthorizer = browsersafe.NewBrowserSafeAuthorizer(accessRestrictionAuthorizer, user.AllAuthenticated)
	}

	kubeAuthorizer := rbacauthorizer.New(
		&rbacauthorizer.RoleGetter{Lister: rbacInformers.Roles().Lister()},
		&rbacauthorizer.RoleBindingLister{Lister: rbacInformers.RoleBindings().Lister()},
		&rbacauthorizer.ClusterRoleGetter{Lister: rbacInformers.ClusterRoles().Lister()},
		&rbacauthorizer.ClusterRoleBindingLister{Lister: rbacInformers.ClusterRoleBindings().Lister()},
	)

	graph := node.NewGraph()
	node.AddGraphEventHandlers(
		graph,
		informers.GetInternalKubeInformers().Core().InternalVersion().Nodes(),
		informers.GetInternalKubeInformers().Core().InternalVersion().Pods(),
		informers.GetInternalKubeInformers().Core().InternalVersion().PersistentVolumes(),
		informers.GetExternalKubeInformers().Storage().V1beta1().VolumeAttachments(),
	)
	nodeAuthorizer := node.NewAuthorizer(graph, nodeidentifier.NewDefaultNodeIdentifier(), kbootstrappolicy.NodeRules())

	return unionFilter(
		// Wrap with an authorizer that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately.
		// Scopes are first because they will authoritatively deny and can logically be attached to anyone.
		browsersafe.NewBrowserSafeAuthorizer(scopeLimitedAuthorizer, user.AllAuthenticated),
		// authorizes system:masters to do anything, just like upstream
		authorizerfactory.NewPrivilegedGroups(user.SystemPrivilegedGroup),
		// The deny authorizer comes after system:masters but before everything else
		// Thus it can never permanently break the cluster because we always have a way to fix things
		denyAuthorizer,
		nodeAuthorizer,
		// Wrap with an authorizer that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately
		browsersafe.NewBrowserSafeAuthorizer(openshiftauthorizer.NewAuthorizer(kubeAuthorizer, messageMaker), user.AllAuthenticated),
	)
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

func unionFilter(authorizers ...authorizer.Authorizer) authorizer.Authorizer {
	return authorizerunion.New(filter(authorizers...)...)
}

func filter(authorizers ...authorizer.Authorizer) []authorizer.Authorizer {
	out := make([]authorizer.Authorizer, 0, len(authorizers))
	for _, authorizer := range authorizers {
		if authorizer != nil {
			out = append(out, authorizer)
		}
	}
	return out
}
