package identitymapper

import (
	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	kuser "k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/sets"

	authapi "github.com/openshift/origin/pkg/auth/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
)

// UserForNewIdentityGetter is responsible for creating or locating the persisted User for the given Identity.
// The preferredUserName is available to the strategies
type UserForNewIdentityGetter interface {
	// UserForNewIdentity returns a persisted User object for the given Identity, creating it if needed
	UserForNewIdentity(ctx kapi.Context, preferredUserName string, identity *userapi.Identity) (*userapi.User, error)
}

var _ = authapi.UserIdentityMapper(&provisioningIdentityMapper{})

// provisioningIdentityMapper implements api.UserIdentityMapper
// If an existing UserIdentityMapping exists for an identity, it is returned.
// If an identity does not exist, it creates an Identity referencing the user returned from provisioningStrategy.UserForNewIdentity
// Otherwise an error is returned
type provisioningIdentityMapper struct {
	identity             identityregistry.Registry
	user                 userregistry.Registry
	provisioningStrategy UserForNewIdentityGetter
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
		// Only retry for the following types of errors:
		// AlreadyExists errors:
		// * The same user was created by another identity provider with the same preferred username
		// * The same user was created by another instance of this identity provider (e.g. double-clicked login button)
		// * The same identity was created by another instance of this identity provider (e.g. double-clicked login button)
		// Conflict errors:
		// * The same user was updated be another identity provider to add identity info
		if (kerrs.IsAlreadyExists(err) || kerrs.IsConflict(err)) && allowedRetries > 0 {
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
	persistedUser, err := p.provisioningStrategy.UserForNewIdentity(ctx, getPreferredUserName(identity), identity)
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
	if !sets.NewString(u.Identities...).Has(identity.Name) {
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
