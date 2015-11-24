package user

import (
	kapi "k8s.io/kubernetes/pkg/api"
	authuser "k8s.io/kubernetes/pkg/auth/user"

	"github.com/openshift/origin/pkg/user/registry/user"
)

// GroupRetriever abstracts returning a list of groups the specified user is in.
type GroupRetriever interface {
	GroupsFor(name string) ([]string, error)
}

// RegistryRetriever retrieves user.Info objects from a user name.
type RegistryRetriever struct {
	users  user.Registry
	groups GroupRetriever

	allowUserErrors  bool
	allowGroupErrors bool
}

// NewRegistryRetriever allows user.Info objects to be retrieved with group info.
// TODO: insert virtual groups for system users
func NewRegistryRetriever(users user.Registry, groups GroupRetriever, allowUserErrors bool, allowGroupErrors bool) *RegistryRetriever {
	return &RegistryRetriever{
		users:  users,
		groups: groups,

		allowUserErrors:  allowUserErrors,
		allowGroupErrors: allowGroupErrors,
	}
}

// User returns information about the provided user or an error. If user or group errors
// are ignored, the remaining info is left empty.
func (r *RegistryRetriever) User(name string) (authuser.Info, error) {
	info := &authuser.DefaultInfo{Name: name}

	user, err := r.users.GetUser(kapi.NewContext(), name)
	if err != nil && !r.allowUserErrors {
		return nil, err
	}
	if user != nil {
		info.Name = user.Name
		info.UID = string(user.UID)
	}

	// use name on the info in case it is different
	groups, err := r.groups.GroupsFor(info.Name)
	if err != nil && !r.allowGroupErrors {
		return nil, err
	}
	info.Groups = groups

	return info, nil
}
