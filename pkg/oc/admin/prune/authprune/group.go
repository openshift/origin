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
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset"
)

func NewGroupReaper(
	userClient userclient.Interface,
	authorizationClient authclient.Interface,
	sccClient securitytypedclient.SecurityContextConstraintsInterface,
) kubectl.Reaper {
	return &GroupReaper{
		userClient:          userClient,
		authorizationClient: authorizationClient,
		sccClient:           sccClient,
	}
}

type GroupReaper struct {
	userClient          userclient.Interface
	authorizationClient authclient.Interface
	sccClient           securitytypedclient.SecurityContextConstraintsInterface
}

// Stop on a reaper is actually used for deletion.  In this case, we'll delete referencing identities, clusterBindings, and bindings,
// then delete the group
func (r *GroupReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *metav1.DeleteOptions) error {
	err := reapForGroup(r.authorizationClient, r.sccClient, name, ioutil.Discard)
	if err != nil {
		glog.Infof("Cannot prune for user/%s: %v", name, err)
	}

	// Remove the group
	if err := r.userClient.User().Groups().Delete(name, &metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
		return err
	}

	return nil
}

func reapForGroup(
	authorizationClient authclient.Interface,
	securityClient securitytypedclient.SecurityContextConstraintsInterface,
	name string,
	out io.Writer) error {

	errors := []error{}

	removedSubject := kapi.ObjectReference{Kind: "Group", Name: name}
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
