package identitymapper

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/user"
	userapi "github.com/openshift/origin/pkg/user/api"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
)

var _ = UserForNewIdentityGetter(&StrategyClaim{})

// StrategyClaim associates a new identity with a user with the identity's preferred username
// if no other identities are already associated with the user
type StrategyClaim struct {
	user        userregistry.Registry
	initializer user.Initializer
}

type claimError struct {
	User     *userapi.User
	Identity *userapi.Identity
}

func IsClaimError(err error) bool {
	_, ok := err.(claimError)
	return ok
}
func NewClaimError(user *userapi.User, identity *userapi.Identity) error {
	return claimError{User: user, Identity: identity}
}

func (c claimError) Error() string {
	return fmt.Sprintf("user %q cannot be claimed by identity %q because it is already mapped to %v", c.User.Name, c.Identity.Name, c.User.Identities)
}

func NewStrategyClaim(user userregistry.Registry, initializer user.Initializer) UserForNewIdentityGetter {
	return &StrategyClaim{user, initializer}
}

func (s *StrategyClaim) UserForNewIdentity(ctx kapi.Context, preferredUserName string, identity *userapi.Identity) (*userapi.User, error) {

	persistedUser, err := s.user.GetUser(ctx, preferredUserName)

	switch {
	case kerrs.IsNotFound(err):
		// Create a new user, propagating any "already exists" errors
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

		// If this user has no other identities, claim, initialize, and update
		if len(persistedUser.Identities) == 0 {
			persistedUser.Identities = []string{identity.Name}
			s.initializer.InitializeUser(identity, persistedUser)
			return s.user.UpdateUser(ctx, persistedUser)
		}

		// Otherwise another identity has already claimed this user, return an error
		return nil, NewClaimError(persistedUser, identity)

	default:
		// Fail on errors other than "not found"
		return nil, err
	}

}
