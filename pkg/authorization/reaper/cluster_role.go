package reaper

import (
	"time"

	"github.com/golang/glog"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl"

	authclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
)

func NewClusterRoleReaper(roleClient authclient.ClusterRolesGetter, clusterBindingClient authclient.ClusterRoleBindingsGetter, bindingClient authclient.RoleBindingsGetter) kubectl.Reaper {
	return &ClusterRoleReaper{
		roleClient:           roleClient,
		clusterBindingClient: clusterBindingClient,
		bindingClient:        bindingClient,
	}
}

type ClusterRoleReaper struct {
	roleClient           authclient.ClusterRolesGetter
	clusterBindingClient authclient.ClusterRoleBindingsGetter
	bindingClient        authclient.RoleBindingsGetter
}

// Stop on a reaper is actually used for deletion.  In this case, we'll delete referencing clusterroleclusterBindings
// then delete the clusterrole.
func (r *ClusterRoleReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *metav1.DeleteOptions) error {
	clusterBindings, err := r.clusterBindingClient.ClusterRoleBindings().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, clusterBinding := range clusterBindings.Items {
		if clusterBinding.RoleRef.Name == name {
			if err := r.clusterBindingClient.ClusterRoleBindings().Delete(clusterBinding.Name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
				glog.Infof("Cannot delete clusterrolebinding/%s: %v", clusterBinding.Name, err)
			}
		}
	}

	namespacedBindings, err := r.bindingClient.RoleBindings(kapi.NamespaceNone).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, namespacedBinding := range namespacedBindings.Items {
		if namespacedBinding.RoleRef.Namespace == kapi.NamespaceNone && namespacedBinding.RoleRef.Name == name {
			if err := r.bindingClient.RoleBindings(namespacedBinding.Namespace).Delete(namespacedBinding.Name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
				glog.Infof("Cannot delete rolebinding/%s in %s: %v", namespacedBinding.Name, namespacedBinding.Namespace, err)
			}
		}
	}

	if err := r.roleClient.ClusterRoles().Delete(name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
		return err
	}

	return nil
}
