package auth

import (
	"fmt"
	"io"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
)

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
