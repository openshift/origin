package reaper

import (
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
)

func NewRoleReaper(roleClient client.RolesNamespacer, bindingClient client.RoleBindingsNamespacer) kubectl.Reaper {
	return &RoleReaper{
		roleClient:    roleClient,
		bindingClient: bindingClient,
	}
}

type RoleReaper struct {
	roleClient    client.RolesNamespacer
	bindingClient client.RoleBindingsNamespacer
}

// Stop on a reaper is actually used for deletion.  In this case, we'll delete referencing rolebindings
// then delete the role.
func (r *RoleReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *kapi.DeleteOptions) (string, error) {
	bindings, err := r.bindingClient.RoleBindings(namespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return "", err
	}

	for _, binding := range bindings.Items {
		if binding.RoleRef.Namespace == namespace && binding.RoleRef.Name == name {
			if err := r.bindingClient.RoleBindings(namespace).Delete(binding.Name); err != nil && !kerrors.IsNotFound(err) {
				glog.Infof("Cannot delete rolebinding/%s: %v", binding.Name, err)
			}
		}
	}

	if err := r.roleClient.Roles(namespace).Delete(name); err != nil && !kerrors.IsNotFound(err) {
		return "", err
	}

	return "", nil
}
