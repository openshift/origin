package authprune

import (
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/golang/glog"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl"
)

func NewClusterRoleReaper(roleClient rbacv1client.ClusterRolesGetter, clusterBindingClient rbacv1client.ClusterRoleBindingsGetter, bindingClient rbacv1client.RoleBindingsGetter) kubectl.Reaper {
	return &ClusterRoleReaper{
		roleClient:           roleClient,
		clusterBindingClient: clusterBindingClient,
		bindingClient:        bindingClient,
	}
}

type ClusterRoleReaper struct {
	roleClient           rbacv1client.ClusterRolesGetter
	clusterBindingClient rbacv1client.ClusterRoleBindingsGetter
	bindingClient        rbacv1client.RoleBindingsGetter
}

// Stop on a reaper is actually used for deletion.  In this case, we'll delete referencing clusterroleclusterBindings
// then delete the clusterrole.
func (r *ClusterRoleReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *metav1.DeleteOptions) error {
	err := reapForClusterRole(r.clusterBindingClient, r.bindingClient, namespace, name, ioutil.Discard)
	if err != nil {
		glog.Infof("Cannot prune for clusterrole/%s: %v", name, err)
	}

	if err := r.roleClient.ClusterRoles().Delete(name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
		return err
	}

	return nil
}

func reapForClusterRole(clusterBindingClient rbacv1client.ClusterRoleBindingsGetter, bindingClient rbacv1client.RoleBindingsGetter, namespace, name string, out io.Writer) error {
	errors := []error{}

	clusterBindings, err := clusterBindingClient.ClusterRoleBindings().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, clusterBinding := range clusterBindings.Items {
		if clusterBinding.RoleRef.Name == name {
			if err := clusterBindingClient.ClusterRoleBindings().Delete(clusterBinding.Name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "clusterrolebinding.rbac.authorization.k8s.io/"+clusterBinding.Name+" deleted\n")
			}
		}
	}

	namespacedBindings, err := bindingClient.RoleBindings(kapi.NamespaceNone).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, namespacedBinding := range namespacedBindings.Items {
		if namespacedBinding.RoleRef.Kind == "ClusterRole" && namespacedBinding.RoleRef.Name == name {
			if err := bindingClient.RoleBindings(namespacedBinding.Namespace).Delete(namespacedBinding.Name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "rolebinding.rbac.authorization.k8s.io/"+namespacedBinding.Name+" deleted\n")
			}
		}
	}

	return utilerrors.NewAggregate(errors)
}
