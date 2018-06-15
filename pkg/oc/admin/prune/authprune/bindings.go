package authprune

import (
	"fmt"
	"io"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	authclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
)

// reapClusterBindings removes the subject from cluster-level role bindings
func reapClusterBindings(removedSubject kapi.ObjectReference, c authclient.Interface, out io.Writer) []error {
	errors := []error{}

	clusterBindings, err := c.Authorization().ClusterRoleBindings().List(metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}
	for _, binding := range clusterBindings.Items {
		retainedSubjects := []kapi.ObjectReference{}
		for _, subject := range binding.Subjects {
			if subject != removedSubject {
				retainedSubjects = append(retainedSubjects, subject)
			}
		}
		if len(retainedSubjects) != len(binding.Subjects) {
			updatedBinding := binding
			updatedBinding.Subjects = retainedSubjects
			if _, err := c.Authorization().ClusterRoleBindings().Update(&updatedBinding); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "clusterrolebinding.rbac.authorization.k8s.io/"+updatedBinding.Name+" updated\n")
			}
		}
	}
	return errors
}

// reapNamespacedBindings removes the subject from namespaced role bindings
func reapNamespacedBindings(removedSubject kapi.ObjectReference, c authclient.Interface, out io.Writer) []error {
	errors := []error{}

	namespacedBindings, err := c.Authorization().RoleBindings(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}
	for _, binding := range namespacedBindings.Items {
		retainedSubjects := []kapi.ObjectReference{}
		for _, subject := range binding.Subjects {
			if subject != removedSubject {
				retainedSubjects = append(retainedSubjects, subject)
			}
		}
		if len(retainedSubjects) != len(binding.Subjects) {
			updatedBinding := binding
			updatedBinding.Subjects = retainedSubjects
			if _, err := c.Authorization().RoleBindings(binding.Namespace).Update(&updatedBinding); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "rolebinding.rbac.authorization.k8s.io/"+updatedBinding.Name+" updated\n")
			}
		}
	}
	return errors
}
