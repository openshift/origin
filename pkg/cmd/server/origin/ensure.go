package origin

import (
	"fmt"
	"io/ioutil"

	"github.com/golang/glog"

	kapierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericapiserver "k8s.io/apiserver/pkg/server"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/rbac"
	kbootstrappolicy "k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy"

	"github.com/openshift/origin/pkg/oc/admin/policy"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicystorage "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy/etcd"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

// ensureComponentAuthorizationRules initializes the cluster policies
func (c *MasterConfig) ensureComponentAuthorizationRules(context genericapiserver.PostStartHookContext) error {
	clusterPolicyStorage, err := clusterpolicystorage.NewREST(c.RESTOptionsGetter)
	if err != nil {
		glog.Errorf("Error creating policy storage: %v", err)
		return nil
	}
	clusterPolicyRegistry := clusterpolicyregistry.NewRegistry(clusterPolicyStorage)
	ctx := apirequest.WithNamespace(apirequest.NewContext(), "")

	if _, err := clusterPolicyRegistry.GetClusterPolicy(ctx, authorizationapi.PolicyName, &metav1.GetOptions{}); kapierror.IsNotFound(err) {
		glog.Infof("No cluster policy found.  Creating bootstrap policy based on: %v", c.Options.PolicyConfig.BootstrapPolicyFile)

		if err := admin.OverwriteBootstrapPolicy(c.RESTOptionsGetter, c.Options.PolicyConfig.BootstrapPolicyFile, admin.CreateBootstrapPolicyFileFullCommand, true, ioutil.Discard); err != nil {
			glog.Errorf("Error creating bootstrap policy: %v", err)
		}

		// these are namespaced, so we can't reconcile them.  Just try to put them in until we work against rbac
		// This only had to hold us until the transition is complete
		// TODO remove this block and use a post-starthook
		// ensure bootstrap namespaced roles are created or reconciled
		for namespace, roles := range kbootstrappolicy.NamespaceRoles() {
			for _, rbacRole := range roles {
				role := &authorizationapi.Role{}
				if err := authorizationapi.Convert_rbac_Role_To_authorization_Role(&rbacRole, role, nil); err != nil {
					utilruntime.HandleError(fmt.Errorf("unable to convert role.%s/%s in %v: %v", rbac.GroupName, rbacRole.Name, namespace, err))
					continue
				}
				if _, err := c.PrivilegedLoopbackOpenShiftClient.Roles(namespace).Create(role); err != nil {
					// don't fail on failures, try to create as many as you can
					utilruntime.HandleError(fmt.Errorf("unable to reconcile role.%s/%s in %v: %v", rbac.GroupName, role.Name, namespace, err))
				}
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
				if _, err := c.PrivilegedLoopbackOpenShiftClient.RoleBindings(namespace).Create(roleBinding); err != nil {
					// don't fail on failures, try to create as many as you can
					utilruntime.HandleError(fmt.Errorf("unable to reconcile rolebinding.%s/%s in %v: %v", rbac.GroupName, roleBinding.Name, namespace, err))
				}
			}
		}

	} else {
		glog.V(2).Infof("Ignoring bootstrap policy file because cluster policy found")
	}

	// Reconcile roles that must exist for the cluster to function
	// Be very judicious about what is placed in this list, since it will be enforced on every server start
	reconcileRoles := &policy.ReconcileClusterRolesOptions{
		RolesToReconcile: []string{bootstrappolicy.DiscoveryRoleName},
		Confirmed:        true,
		Union:            true,
		Out:              ioutil.Discard,
		RoleClient:       c.PrivilegedLoopbackOpenShiftClient.ClusterRoles(),
	}
	if err := reconcileRoles.RunReconcileClusterRoles(nil, nil); err != nil {
		glog.Errorf("Could not auto reconcile roles: %v\n", err)
	}

	// Reconcile rolebindings that must exist for the cluster to function
	// Be very judicious about what is placed in this list, since it will be enforced on every server start
	reconcileRoleBindings := &policy.ReconcileClusterRoleBindingsOptions{
		RolesToReconcile:  []string{bootstrappolicy.DiscoveryRoleName},
		Confirmed:         true,
		Union:             true,
		Out:               ioutil.Discard,
		RoleBindingClient: c.PrivilegedLoopbackOpenShiftClient.ClusterRoleBindings(),
	}
	if err := reconcileRoleBindings.RunReconcileClusterRoleBindings(nil, nil); err != nil {
		glog.Errorf("Could not auto reconcile role bindings: %v\n", err)
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
