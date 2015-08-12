package identitymapper

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	kuser "k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/user"
	userapi "github.com/openshift/origin/pkg/user/api"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
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

type provisioningIdentityMapper struct {
	identity    identityregistry.Registry
	user        userregistry.Registry
	generator   UserNameGenerator
	initializer user.Initializer
}

// NewAlwaysCreateUserIdentityToUserMapper returns an IdentityMapper that does the following:
// 1. Returns an existing user if the identity exists and is associated with an existing user
// 2. Returns an error if the identity exists and is not associated with a user
// 3. Creates the identity and creates and returns a new user with a unique username if the identity does not yet exist
func NewAlwaysCreateUserIdentityToUserMapper(identityRegistry identityregistry.Registry, userRegistry userregistry.Registry) authapi.UserIdentityMapper {
	return &provisioningIdentityMapper{identityRegistry, userRegistry, DefaultGenerator, user.NewDefaultUserInitStrategy()}
}

// UserFor returns info about the user for whom identity info have been provided
func (p *provisioningIdentityMapper) UserFor(info authapi.UserIdentityInfo) (kuser.Info, error) {
	// Retrying up to three times lets us handle race conditions with up to two conflicting identity providers without returning an error
	// * A single race is possible on user creation for every conflicting identity provider
	// * A single race is possible on user creation between two instances of the same provider
	// * A single race is possible on identity creation between two instances of the same provider
	//
	// A race condition between three conflicting identity providers *and* multiple instances of the same identity provider
	// seems like a reasonable situation to return an error (you would get an AlreadyExists error on either the user or the identity)
	return p.userForWithRetries(info, 3)
}

func (p *provisioningIdentityMapper) userForWithRetries(info authapi.UserIdentityInfo, allowedRetries int) (kuser.Info, error) {
	ctx := kapi.NewContext()

	identity, err := p.identity.GetIdentity(ctx, info.GetIdentityName())

	if kerrs.IsNotFound(err) {
		user, err := p.createIdentityAndMapping(ctx, info)
		// Only retry for AlreadyExists errors, which can occur in the following cases:
		// * The same user was created by another identity provider with the same preferred username
		// * The same user was created by another instance of this identity provider
		// * The same identity was created by another instance of this identity provider
		if kerrs.IsAlreadyExists(err) && allowedRetries > 0 {
			return p.userForWithRetries(info, allowedRetries-1)
		}
		return user, err
	}

	if err != nil {
		return nil, err
	}

	return p.getMapping(ctx, identity)
}

// createIdentityAndMapping creates an identity with a valid user reference for the given identity info
func (p *provisioningIdentityMapper) createIdentityAndMapping(ctx kapi.Context, info authapi.UserIdentityInfo) (kuser.Info, error) {
	// Build the part of the identity we know about
	identity := &userapi.Identity{
		ObjectMeta: kapi.ObjectMeta{
			Name: info.GetIdentityName(),
		},
		ProviderName:     info.GetProviderName(),
		ProviderUserName: info.GetProviderUserName(),
		Extra:            info.GetExtra(),
	}

	// Get or create a persisted user pointing to the identity
	persistedUser, err := p.getOrCreateUserForIdentity(ctx, identity)
	if err != nil {
		return nil, err
	}

	// Create the identity pointing to the persistedUser
	identity.User = kapi.ObjectReference{
		Name: persistedUser.Name,
		UID:  persistedUser.UID,
	}
	if _, err := p.identity.CreateIdentity(ctx, identity); err != nil {
		return nil, err
	}

	return &kuser.DefaultInfo{
		Name:   persistedUser.Name,
		UID:    string(persistedUser.UID),
		Groups: persistedUser.Groups,
	}, nil
}

func (p *provisioningIdentityMapper) getOrCreateUserForIdentity(ctx kapi.Context, identity *userapi.Identity) (*userapi.User, error) {

	preferredUserName := getPreferredUserName(identity)

	// Iterate through the max allowed generated usernames
	// If an existing user references this identity, associate the identity with that user and return
	// Otherwise, create a user with the first generated user name that does not already exist and return.
	// Names are created in a deterministic order, so the first one that isn't present gets created.
	// In the case of a race, one will get to persist the user object and the other will fail.
	for sequence := 0; sequence < MaxGenerateAttempts; sequence++ {
		// Get the username we want
		potentialUserName := p.generator(preferredUserName, sequence)

		// See if it already exists
		persistedUser, err := p.user.GetUser(ctx, potentialUserName)

		if err != nil && !kerrs.IsNotFound(err) {
			// Fail on errors other than "not found"
			return nil, err
		}

		if err != nil && kerrs.IsNotFound(err) {
			// Try to create a user with the available name
			desiredUser := &userapi.User{
				ObjectMeta: kapi.ObjectMeta{Name: potentialUserName},
				Identities: []string{identity.Name},
			}

			// Initialize from the identity
			p.initializer.InitializeUser(identity, desiredUser)

			// Create the user
			createdUser, err := p.user.CreateUser(ctx, desiredUser)
			if err != nil {
				return nil, err
			}
			return createdUser, nil
		}

		if util.NewStringSet(persistedUser.Identities...).Has(identity.Name) {
			// If the existing user references our identity, we're done
			return persistedUser, nil
		}
	}

	return nil, errors.New("Could not create user, max attempts exceeded")
}

func (p *provisioningIdentityMapper) getMapping(ctx kapi.Context, identity *userapi.Identity) (kuser.Info, error) {
	if len(identity.User.Name) == 0 {
		return nil, kerrs.NewNotFound("UserIdentityMapping", identity.Name)
	}
	u, err := p.user.GetUser(ctx, identity.User.Name)
	if err != nil {
		return nil, err
	}
	if u.UID != identity.User.UID {
		glog.Errorf("identity.user.uid (%s) and user.uid (%s) do not match for identity %s", identity.User.UID, u.UID, identity.Name)
		return nil, kerrs.NewNotFound("UserIdentityMapping", identity.Name)
	}
	if !util.NewStringSet(u.Identities...).Has(identity.Name) {
		glog.Errorf("user.identities (%#v) does not include identity (%s)", u, identity.Name)
		return nil, kerrs.NewNotFound("UserIdentityMapping", identity.Name)
	}
	return &kuser.DefaultInfo{
		Name:   u.Name,
		UID:    string(u.UID),
		Groups: u.Groups,
	}, nil
}

func getPreferredUserName(identity *userapi.Identity) string {
	if login, ok := identity.Extra[authapi.IdentityPreferredUsernameKey]; ok && len(login) > 0 {
		return login
	}
	return identity.ProviderUserName
}
