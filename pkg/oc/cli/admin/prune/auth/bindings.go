package auth

import (
	"fmt"
	"io"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authv1client "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
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
			updatedBinding.UserNames, updatedBinding.GroupNames = stringSubjectsFor(binding.Namespace, retainedSubjects)
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
			updatedBinding.UserNames, updatedBinding.GroupNames = stringSubjectsFor(binding.Namespace, retainedSubjects)
			if _, err := c.RoleBindings(binding.Namespace).Update(&updatedBinding); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "rolebinding.rbac.authorization.k8s.io/"+updatedBinding.Name+" updated\n")
			}
		}
	}
	return errors
}

// stringSubjectsFor returns users and groups for comparison against user.Info.  currentNamespace is used to
// to create usernames for service accounts where namespace=="".
func stringSubjectsFor(currentNamespace string, subjects []corev1.ObjectReference) ([]string, []string) {
	// these MUST be nil to indicate empty
	var users, groups []string

	for _, subject := range subjects {
		switch subject.Kind {
		case "ServiceAccount":
			namespace := currentNamespace
			if len(subject.Namespace) > 0 {
				namespace = subject.Namespace
			}
			if len(namespace) > 0 {
				users = append(users, serviceaccount.MakeUsername(namespace, subject.Name))
			}

		case "User":
			users = append(users, subject.Name)

		case "Group":
			groups = append(groups, subject.Name)
		}
	}

	return users, groups
}
