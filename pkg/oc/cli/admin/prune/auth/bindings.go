package auth

import (
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/core"
	corev1conversions "k8s.io/kubernetes/pkg/apis/core/v1"

	authv1client "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

// reapClusterBindings removes the subject from cluster-level role bindings
func reapClusterBindings(removedSubject corev1.ObjectReference, c authv1client.AuthorizationV1Interface, out io.Writer) []error {
	errors := []error{}

	clusterBindings, err := c.ClusterRoleBindings().List(metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}
	for _, binding := range clusterBindings.Items {
		retainedSubjects := []corev1.ObjectReference{}
		for _, subject := range binding.Subjects {
			if subject != removedSubject {
				retainedSubjects = append(retainedSubjects, subject)
			}
		}
		if len(retainedSubjects) != len(binding.Subjects) {
			updatedBinding := binding
			updatedBinding.Subjects = retainedSubjects
			coreSubjects, err := convertObjectReference(retainedSubjects)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			updatedBinding.UserNames, updatedBinding.GroupNames = authorizationapi.StringSubjectsFor(binding.Namespace, coreSubjects)
			if _, err := c.ClusterRoleBindings().Update(&updatedBinding); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "clusterrolebinding.rbac.authorization.k8s.io/"+updatedBinding.Name+" updated\n")
			}
		}
	}
	return errors
}

// reapNamespacedBindings removes the subject from namespaced role bindings
func reapNamespacedBindings(removedSubject corev1.ObjectReference, c authv1client.AuthorizationV1Interface, out io.Writer) []error {
	errors := []error{}

	namespacedBindings, err := c.RoleBindings(metav1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return []error{err}
	}
	for _, binding := range namespacedBindings.Items {
		retainedSubjects := []corev1.ObjectReference{}
		for _, subject := range binding.Subjects {
			if subject != removedSubject {
				retainedSubjects = append(retainedSubjects, subject)
			}
		}
		if len(retainedSubjects) != len(binding.Subjects) {
			updatedBinding := binding
			updatedBinding.Subjects = retainedSubjects
			coreSubjects, err := convertObjectReference(retainedSubjects)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			updatedBinding.UserNames, updatedBinding.GroupNames = authorizationapi.StringSubjectsFor(binding.Namespace, coreSubjects)
			if _, err := c.RoleBindings(binding.Namespace).Update(&updatedBinding); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "rolebinding.rbac.authorization.k8s.io/"+updatedBinding.Name+" updated\n")
			}
		}
	}
	return errors
}

func convertObjectReference(ins []corev1.ObjectReference) ([]core.ObjectReference, error) {
	result := []core.ObjectReference{}
	for _, subject := range ins {
		ref := &core.ObjectReference{}
		if err := corev1conversions.Convert_v1_ObjectReference_To_core_ObjectReference(&subject, ref, nil); err != nil {
			return nil, err
		}
		result = append(result, *ref)
	}
	return result, nil
}
