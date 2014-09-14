package etcd

import (
	"errors"
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/openshift/origin/pkg/user"
	"github.com/openshift/origin/pkg/user/api"
)

// Etcd implements UserIdentityMapping backed by etcd.
type Etcd struct {
	tools.EtcdHelper
	initializer user.Initializer
}

// New returns a new Etcd.
func New(helper tools.EtcdHelper, initializer user.Initializer) *Etcd {
	return &Etcd{
		EtcdHelper:  helper,
		initializer: initializer,
	}
}

var errExists = errors.New("the mapping already exists")

func makeUserKey(id string) string {
	return "/users/" + id
}

func (r *Etcd) GetUser(name string) (user *api.User, err error) {
	mapping := &api.UserIdentityMapping{}
	err = r.ExtractObj(makeUserKey(name), mapping, false)
	user = &mapping.User
	return
}

// GetOrCreateUserIdentityMapping implements useridentitymapping.Registry
func (r *Etcd) GetOrCreateUserIdentityMapping(mapping *api.UserIdentityMapping) (*api.UserIdentityMapping, error) {
	name := fmt.Sprintf("%s:%s", mapping.Identity.Provider, mapping.Identity.Name)
	key := makeUserKey(name)

	// track the object we set into etcd to return
	var found *api.UserIdentityMapping

	err := r.AtomicUpdate(key, &api.UserIdentityMapping{}, func(in runtime.Object) (runtime.Object, error) {
		existing := *in.(*api.UserIdentityMapping)

		// did not previously exist
		if existing.Identity.Name == "" {
			if err := r.initializer.InitializeUser(&mapping.Identity, &existing.User); err != nil {
				return in, err
			}
			existing.User.Name = name
			existing.Identity = mapping.Identity
			found = &existing
			return &existing, nil
		}

		if existing.User.Name != name {
			return in, fmt.Errorf("the provided user name does not match the existing mapping %s", existing.User.Name)
		}
		found = &existing

		// TODO: determine whether to update identity
		return in, errExists
	})

	if err != nil && err != errExists {
		return nil, err
	}
	return found, nil
}
