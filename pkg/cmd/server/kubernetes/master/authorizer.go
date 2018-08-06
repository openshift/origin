package master

import (
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	authorizerunion "k8s.io/apiserver/pkg/authorization/union"
	kinformers "k8s.io/client-go/informers"
	"k8s.io/kubernetes/pkg/auth/nodeidentifier"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/plugin/pkg/auth/authorizer/node"
	rbacauthorizer "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac"
	kbootstrappolicy "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"

	openshiftauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/authorizer/browsersafe"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
)

func NewAuthorizer(internalInformers kinternalinformers.SharedInformerFactory, informers kinformers.SharedInformerFactory, projectRequestDenyMessage string) authorizer.Authorizer {
	messageMaker := openshiftauthorizer.NewForbiddenMessageResolver(projectRequestDenyMessage)
	rbacInformers := informers.Rbac().V1()

	scopeLimitedAuthorizer := scope.NewAuthorizer(rbacInformers.ClusterRoles().Lister(), messageMaker)

	kubeAuthorizer := rbacauthorizer.New(
		&rbacauthorizer.RoleGetter{Lister: rbacInformers.Roles().Lister()},
		&rbacauthorizer.RoleBindingLister{Lister: rbacInformers.RoleBindings().Lister()},
		&rbacauthorizer.ClusterRoleGetter{Lister: rbacInformers.ClusterRoles().Lister()},
		&rbacauthorizer.ClusterRoleBindingLister{Lister: rbacInformers.ClusterRoleBindings().Lister()},
	)

	graph := node.NewGraph()
	node.AddGraphEventHandlers(
		graph,
		internalInformers.Core().InternalVersion().Nodes(),
		internalInformers.Core().InternalVersion().Pods(),
		internalInformers.Core().InternalVersion().PersistentVolumes(),
		informers.Storage().V1beta1().VolumeAttachments(),
	)
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
