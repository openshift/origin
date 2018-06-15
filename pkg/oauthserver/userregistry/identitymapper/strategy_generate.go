package identitymapper

import (
	"errors"
	"fmt"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	userapi "github.com/openshift/api/user/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
)

// UserNameGenerator returns a username
type UserNameGenerator func(base string, sequence int) string

var (
	// MaxGenerateAttempts limits how many times we try to find an available username for a new identity
	MaxGenerateAttempts = 100

	// DefaultGenerator attempts to use the base name first, then "base2", "base3", ...
	DefaultGenerator = UserNameGenerator(func(base string, sequence int) string {
		if sequence == 0 {
			return base
		}
		return fmt.Sprintf("%s%d", base, sequence+1)
	})
)

var _ = UserForNewIdentityGetter(&StrategyGenerate{})

// StrategyGenerate finds an available username for a new identity, based on its preferred username
// If a user with the preferred username already exists, a unique username is generated
type StrategyGenerate struct {
	user        userclient.UserInterface
	generator   UserNameGenerator
	initializer Initializer
}

func NewStrategyGenerate(user userclient.UserInterface, initializer Initializer) UserForNewIdentityGetter {
	return &StrategyGenerate{user, DefaultGenerator, initializer}
}

func (s *StrategyGenerate) UserForNewIdentity(ctx apirequest.Context, preferredUserName string, identity *userapi.Identity) (*userapi.User, error) {

	// Iterate through the max allowed generated usernames
	// If an existing user references this identity, associate the identity with that user and return
	// Otherwise, create a user with the first generated user name that does not already exist and return.
	// Names are created in a deterministic order, so the first one that isn't present gets created.
	// In the case of a race, one will get to persist the user object and the other will fail.
UserSearch:
	for sequence := 0; sequence < MaxGenerateAttempts; sequence++ {
		// GetUsers the username we want
		potentialUserName := s.generator(preferredUserName, sequence)

		// See if it already exists
		persistedUser, err := s.user.Get(potentialUserName, metav1.GetOptions{})

		switch {
		case kerrs.IsNotFound(err):
			// CreateUser a new user
			desiredUser := &userapi.User{}
			desiredUser.Name = potentialUserName
			desiredUser.Identities = []string{identity.Name}
			s.initializer.InitializeUser(identity, desiredUser)
			return s.user.Create(desiredUser)

		case err == nil:
			// If the existing user already references our identity, we're done
			if sets.NewString(persistedUser.Identities...).Has(identity.Name) {
				return persistedUser, nil
			}
			// Otherwise, continue our search for a user
			continue UserSearch

		default:
			// Fail on errors other than "not found"
			return nil, err
		}
	}

	return nil, errors.New("Could not create user, max attempts exceeded")
}
