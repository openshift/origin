package identitymapper

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/user"
	userapi "github.com/openshift/origin/pkg/user/api"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
)

var _ = UserForNewIdentityGetter(&StrategyAdd{})

// StrategyAdd associates a new identity with a user with the identity's preferred username,
// adding to any existing identities associated with the user
type StrategyAdd struct {
	user        userregistry.Registry
	initializer user.Initializer
}

func NewStrategyAdd(user userregistry.Registry, initializer user.Initializer) UserForNewIdentityGetter {
	return &StrategyAdd{user, initializer}
}

func (s *StrategyAdd) UserForNewIdentity(ctx kapi.Context, preferredUserName string, identity *userapi.Identity) (*userapi.User, error) {

	persistedUser, err := s.user.GetUser(ctx, preferredUserName)

	switch {
	case kerrs.IsNotFound(err):
		// Create a new user
		desiredUser := &userapi.User{}
		desiredUser.Name = preferredUserName
		desiredUser.Identities = []string{identity.Name}
		s.initializer.InitializeUser(identity, desiredUser)
		return s.user.CreateUser(ctx, desiredUser)

	case err == nil:
		// If the existing user already references our identity, we're done
		if sets.NewString(persistedUser.Identities...).Has(identity.Name) {
			return persistedUser, nil
		}

		// Otherwise add our identity and update
		persistedUser.Identities = append(persistedUser.Identities, identity.Name)
		// If our newly added identity is the only one, initialize the user
		if len(persistedUser.Identities) == 1 {
			s.initializer.InitializeUser(identity, persistedUser)
		}
		return s.user.UpdateUser(ctx, persistedUser)

	default:
		// Fail on errors other than "not found"
		return nil, err
	}
}
