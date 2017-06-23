package controller

import (
	rbacinternalversion "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"

	"github.com/openshift/origin/pkg/authorization/controller/authorizationsync"
)

type OriginToRBACSyncControllerConfig struct {
	PrivilegedRBACClient rbacinternalversion.RbacInterface
}

func (c *OriginToRBACSyncControllerConfig) RunController(ctx ControllerContext) (bool, error) {
	rbacInformer := ctx.InternalKubeInformers.Rbac().InternalVersion()
	authInformer := ctx.AuthorizationInformers.Authorization().InternalVersion()

	clusterRoles := authorizationsync.NewOriginToRBACClusterRoleController(
		rbacInformer.ClusterRoles(),
		authInformer.ClusterPolicies(),
		c.PrivilegedRBACClient,
	)
	go clusterRoles.Run(5, ctx.Stop)
	clusterRoleBindings := authorizationsync.NewOriginToRBACClusterRoleBindingController(
		rbacInformer.ClusterRoleBindings(),
		authInformer.ClusterPolicyBindings(),
		c.PrivilegedRBACClient,
	)
	go clusterRoleBindings.Run(5, ctx.Stop)

	roles := authorizationsync.NewOriginToRBACRoleController(
		rbacInformer.Roles(),
		authInformer.Policies(),
		c.PrivilegedRBACClient,
	)
	go roles.Run(5, ctx.Stop)
	roleBindings := authorizationsync.NewOriginToRBACRoleBindingController(
		rbacInformer.RoleBindings(),
		authInformer.PolicyBindings(),
		c.PrivilegedRBACClient,
	)
	go roleBindings.Run(5, ctx.Stop)

	return true, nil
}
