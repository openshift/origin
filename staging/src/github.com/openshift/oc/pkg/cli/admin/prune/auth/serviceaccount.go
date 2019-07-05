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

func reapForServiceAccount(
	authorizationClient authv1client.AuthorizationV1Interface,
	securityClient securityv1client.SecurityContextConstraintsInterface,
	nsname string,
	name string,
	out io.Writer) error {

	errors := []error{}

	removedSubject := corev1.ObjectReference{Kind: "ServiceAccount", Name: name, Namespace: nsname}
	errors = append(errors, reapClusterBindings(removedSubject, authorizationClient, out)...)
	errors = append(errors, reapNamespacedBindings(removedSubject, authorizationClient, out)...)

	// Remove the sa from sccs
	sccs, err := securityClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	saname := "system:serviceaccount:" + nsname + ":" + name
	for _, scc := range sccs.Items {
		retainedUsers := []string{}
		for _, user := range scc.Users {
			if user != saname {
				retainedUsers = append(retainedUsers, user)
			}
		}
		if len(retainedUsers) != len(scc.Users) {
			updatedSCC := scc
			updatedSCC.Users = retainedUsers
			if _, err := securityClient.Update(&updatedSCC); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "securitycontextconstraints.security.openshift.io/"+updatedSCC.Name+" updated\n")
			}
		}
	}

	return utilerrors.NewAggregate(errors)
}
