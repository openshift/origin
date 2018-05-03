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
	"k8s.io/kubernetes/pkg/kubectl"
)

func NewRoleReaper(roleClient rbacv1client.RolesGetter, bindingClient rbacv1client.RoleBindingsGetter) kubectl.Reaper {
	return &RoleReaper{
		roleClient:    roleClient,
		bindingClient: bindingClient,
	}
}

type RoleReaper struct {
	roleClient    rbacv1client.RolesGetter
	bindingClient rbacv1client.RoleBindingsGetter
}

// Stop on a reaper is actually used for deletion.  In this case, we'll delete referencing rolebindings
// then delete the role.
func (r *RoleReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *metav1.DeleteOptions) error {
	err := reapForRole(r.bindingClient, namespace, name, ioutil.Discard)
	if err != nil {
		glog.Infof("Cannot prune for role/%s: %v", name, err)
	}

	if err := r.roleClient.Roles(namespace).Delete(name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
		return err
	}

	return nil
}

func reapForRole(bindingClient rbacv1client.RoleBindingsGetter, namespace, name string, out io.Writer) error {
	bindings, err := bindingClient.RoleBindings(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	errors := []error{}
	for _, binding := range bindings.Items {
		if binding.RoleRef.Kind == "Role" && binding.RoleRef.Name == name {
			foreground := metav1.DeletePropagationForeground
			if err := bindingClient.RoleBindings(namespace).Delete(binding.Name, &metav1.DeleteOptions{PropagationPolicy: &foreground}); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "rolebinding.rbac.authorization.k8s.io/"+binding.Name+" deleted\n")
			}
		}
	}

	return utilerrors.NewAggregate(errors)
}
