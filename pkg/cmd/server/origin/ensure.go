package origin

import (
	"fmt"
	"io/ioutil"

	"github.com/golang/glog"

	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/rbac"
	kbootstrappolicy "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"

	"github.com/openshift/origin/pkg/oc/admin/policy"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

// ensureComponentAuthorizationRules initializes the cluster policies
func (c *MasterConfig) ensureComponentAuthorizationRules(context genericapiserver.PostStartHookContext) error {
	reconcileRole := &policy.ReconcileClusterRolesOptions{
		RolesToReconcile: nil, // means all
		Confirmed:        true,
		Union:            true,
		Out:              ioutil.Discard,
		RoleClient:       c.PrivilegedLoopbackOpenShiftClient.ClusterRoles(),
	}
	if err := reconcileRole.RunReconcileClusterRoles(nil, nil); err != nil {
		glog.Errorf("Could not reconcile: %v", err)
	}
	reconcileRoleBinding := &policy.ReconcileClusterRoleBindingsOptions{
		RolesToReconcile:  nil, // means all
		Confirmed:         true,
		Union:             true,
		Out:               ioutil.Discard,
		RoleBindingClient: c.PrivilegedLoopbackOpenShiftClient.ClusterRoleBindings(),
	}
	if err := reconcileRoleBinding.RunReconcileClusterRoleBindings(nil, nil); err != nil {
		glog.Errorf("Could not reconcile: %v", err)
	}

	// these are namespaced, so we can't reconcile them.  Just try to put them in until we work against rbac
	// This only had to hold us until the transition is complete
	// ensure bootstrap namespaced roles are created or reconciled
	for namespace, roles := range kbootstrappolicy.NamespaceRoles() {
		for _, rbacRole := range roles {
			role := &authorizationapi.Role{}
			if err := authorizationapi.Convert_rbac_Role_To_authorization_Role(&rbacRole, role, nil); err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to convert role.%s/%s in %v: %v", rbac.GroupName, rbacRole.Name, namespace, err))
				continue
			}
			if _, err := c.PrivilegedLoopbackOpenShiftClient.Roles(namespace).Create(role); err != nil && !kapierror.IsAlreadyExists(err) {
				// don't fail on failures, try to create as many as you can
				utilruntime.HandleError(fmt.Errorf("unable to reconcile role.%s/%s in %v: %v", rbac.GroupName, role.Name, namespace, err))
			}
		}
	}
	for _, role := range bootstrappolicy.GetBootstrapOpenshiftRoles(c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace) {
		if _, err := c.PrivilegedLoopbackOpenShiftClient.Roles(c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace).Create(&role); err != nil && !kapierror.IsAlreadyExists(err) {
			// don't fail on failures, try to create as many as you can
			utilruntime.HandleError(fmt.Errorf("unable to reconcile role.%s/%s in %v: %v", rbac.GroupName, role.Name, c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace, err))
		}
	}

	// ensure bootstrap namespaced rolebindings are created or reconciled
	for namespace, roleBindings := range kbootstrappolicy.NamespaceRoleBindings() {
		for _, rbacRoleBinding := range roleBindings {
			roleBinding := &authorizationapi.RoleBinding{}
			if err := authorizationapi.Convert_rbac_RoleBinding_To_authorization_RoleBinding(&rbacRoleBinding, roleBinding, nil); err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to convert rolebinding.%s/%s in %v: %v", rbac.GroupName, rbacRoleBinding.Name, namespace, err))
				continue
			}
			if _, err := c.PrivilegedLoopbackOpenShiftClient.RoleBindings(namespace).Create(roleBinding); err != nil && !kapierror.IsAlreadyExists(err) {
				// don't fail on failures, try to create as many as you can
				utilruntime.HandleError(fmt.Errorf("unable to reconcile rolebinding.%s/%s in %v: %v", rbac.GroupName, roleBinding.Name, namespace, err))
			}
		}
	}
	for _, roleBinding := range bootstrappolicy.GetBootstrapOpenshiftRoleBindings(c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace) {
		if _, err := c.PrivilegedLoopbackOpenShiftClient.RoleBindings(c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace).Create(&roleBinding); err != nil && !kapierror.IsAlreadyExists(err) {
			// don't fail on failures, try to create as many as you can
			utilruntime.HandleError(fmt.Errorf("unable to reconcile rolebinding.%s/%s in %v: %v", rbac.GroupName, roleBinding.Name, c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace, err))
		}
	}

	return nil
}

// ensureOpenShiftSharedResourcesNamespace is called as part of global policy initialization to ensure shared namespace exists
func (c *MasterConfig) ensureOpenShiftSharedResourcesNamespace(context genericapiserver.PostStartHookContext) error {
	if _, err := c.PrivilegedLoopbackKubernetesClientsetInternal.Core().Namespaces().Get(c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace, metav1.GetOptions{}); kapierror.IsNotFound(err) {
		namespace, createErr := c.PrivilegedLoopbackKubernetesClientsetInternal.Core().Namespaces().Create(&kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace}})
		if createErr != nil {
			glog.Errorf("Error creating namespace: %v due to %v\n", c.Options.PolicyConfig.OpenShiftSharedResourcesNamespace, createErr)
			return nil
		}

		EnsureNamespaceServiceAccountRoleBindings(c.PrivilegedLoopbackKubernetesClientsetInternal, c.PrivilegedLoopbackOpenShiftClient, namespace)
	}
	return nil
}
