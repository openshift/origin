package auth

import (
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	authv1client "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	securityv1client "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1"
)

func reapForGroup(
	authorizationClient authv1client.AuthorizationV1Interface,
	securityClient securityv1client.SecurityContextConstraintsInterface,
	name string,
	out io.Writer) error {

	errors := []error{}

	removedSubject := corev1.ObjectReference{Kind: "Group", Name: name}
	errors = append(errors, reapClusterBindings(removedSubject, authorizationClient, out)...)
	errors = append(errors, reapNamespacedBindings(removedSubject, authorizationClient, out)...)

	// Remove the group from sccs
	sccs, err := securityClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, scc := range sccs.Items {
		retainedGroups := []string{}
		for _, group := range scc.Groups {
			if group != name {
				retainedGroups = append(retainedGroups, group)
			}
		}
		if len(retainedGroups) != len(scc.Groups) {
			updatedSCC := scc
			updatedSCC.Groups = retainedGroups
			if _, err := securityClient.Update(&updatedSCC); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "securitycontextconstraints.security.openshift.io/"+updatedSCC.Name+" updated\n")
			}
		}
	}

	// Intentionally leave identities that reference the user
	// The user does not "own" the identities
	// If the admin wants to remove the identities, that is a distinct operation

	return utilerrors.NewAggregate(errors)
}
