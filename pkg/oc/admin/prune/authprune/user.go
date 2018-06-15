package authprune

import (
	"fmt"
	"io"
	"io/ioutil"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl"

	"github.com/golang/glog"
	authclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
)

func NewUserReaper(
	userClient userclient.Interface,
	authorizationClient authclient.Interface,
	oauthClient oauthclient.Interface,
	sccClient securitytypedclient.SecurityContextConstraintsInterface,
) kubectl.Reaper {
	return &UserReaper{
		userClient:          userClient,
		authorizationClient: authorizationClient,
		oauthClient:         oauthClient,
		sccClient:           sccClient,
	}
}

type UserReaper struct {
	userClient          userclient.Interface
	authorizationClient authclient.Interface
	oauthClient         oauthclient.Interface
	sccClient           securitytypedclient.SecurityContextConstraintsInterface
}

// Stop on a reaper is actually used for deletion.  In this case, we'll delete referencing identities, clusterBindings, and bindings,
// then delete the user
func (r *UserReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *metav1.DeleteOptions) error {
	err := reapForUser(r.userClient, r.authorizationClient, r.oauthClient, r.sccClient, name, ioutil.Discard)
	if err != nil {
		glog.Infof("Cannot prune for user/%s: %v", name, err)
	}

	// Remove the user
	if err := r.userClient.User().Users().Delete(name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
		return err
	}

	return nil
}

func reapForUser(
	userClient userclient.Interface,
	authorizationClient authclient.Interface,
	oauthClient oauthclient.Interface,
	securityClient securitytypedclient.SecurityContextConstraintsInterface,
	name string,
	out io.Writer) error {

	errors := []error{}

	removedSubject := kapi.ObjectReference{Kind: "User", Name: name}
	errors = append(errors, reapClusterBindings(removedSubject, authorizationClient, out)...)
	errors = append(errors, reapNamespacedBindings(removedSubject, authorizationClient, out)...)

	// Remove the user from sccs
	sccs, err := securityClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, scc := range sccs.Items {
		retainedUsers := []string{}
		for _, user := range scc.Users {
			if user != name {
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

	// Remove the user from groups
	groups, err := userClient.User().Groups().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, group := range groups.Items {
		retainedUsers := []string{}
		for _, user := range group.Users {
			if user != name {
				retainedUsers = append(retainedUsers, user)
			}
		}
		if len(retainedUsers) != len(group.Users) {
			updatedGroup := group
			updatedGroup.Users = retainedUsers
			if _, err := userClient.User().Groups().Update(&updatedGroup); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "group.user.openshift.io/"+updatedGroup.Name+" updated\n")
			}
		}
	}

	// Remove the user's OAuthClientAuthorizations
	// Once https://github.com/kubernetes/kubernetes/pull/28112 is fixed, use a field selector
	// to filter on the userName, rather than fetching all authorizations and filtering client-side
	authorizations, err := oauthClient.Oauth().OAuthClientAuthorizations().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, authorization := range authorizations.Items {
		if authorization.UserName == name {
			if err := oauthClient.Oauth().OAuthClientAuthorizations().Delete(authorization.Name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
				errors = append(errors, err)
			} else {
				fmt.Fprintf(out, "oauthclientauthorization.oauth.openshift.io/"+authorization.Name+" updated\n")
			}
		}
	}

	// Intentionally leave identities that reference the user
	// The user does not "own" the identities
	// If the admin wants to remove the identities, that is a distinct operation

	return utilerrors.NewAggregate(errors)
}
