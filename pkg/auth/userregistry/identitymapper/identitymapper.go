package syncgroups

import (
	"errors"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kuser "github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	ouser "github.com/openshift/origin/pkg/user"
	userapi "github.com/openshift/origin/pkg/user/api"
	identityregistry "github.com/openshift/origin/pkg/user/registry/identity"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
)

// DeterministicUserIdentityMapper is a UserIdentityMapper that forces a deterministic mapping from
// an Identity to a User with the following behavior:
// 1. Returns an error if the identity exists and is not associated with a user
// 2. Creates the identity if the identity does not yet exist and:
//   a. searches for the matching OpenShift User deterministically
//     i. returns the existing user if the identity matches
//     ii. optionally adds the identity to the existing User and returns the existing user if identity
//         collisions are allowed and the identity of the existing user does not match
//     iii. optionally creates and returns a new user if the search does not find a User
type DeterministicUserIdentityMapper struct {
	// AllowIdentityCollisions determines if this UserIdentityMapper is allowed to make a mapping
	// between an Identity provided by LDAP and a User that already is mapped to another Identity.
	AllowIdentityCollisions bool
	// ProvisionUsers determines behavior when no User record is found for an Identity. A true value
	// means that if the OpenShift User deterministically mapped to from the Identity does not yet exist,
	// the DeterministicUserIdentityMapper will create it.
	ProvisionUsers bool

	identityRegistry identityregistry.Registry
	userRegistry     userregistry.Registry
	initializer      ouser.Initializer
}

// NewDeterministicUserIdentityToUserMapper returns a DeterminisicUserIdentityMapper
func NewDeterministicUserIdentityToUserMapper(identityRegistry identityregistry.Registry,
	userRegistry userregistry.Registry,
	allowIdentityCollisions bool,
	provisionUsers bool) authapi.UserIdentityMapper {
	return &DeterministicUserIdentityMapper{
		identityRegistry:        identityRegistry,
		userRegistry:            userRegistry,
		initializer:             ouser.NewDefaultUserInitStrategy(),
		AllowIdentityCollisions: allowIdentityCollisions,
		ProvisionUsers:          provisionUsers,
	}
}

// UserFor returns info about the user for whom identity info have been provided
func (d *DeterministicUserIdentityMapper) UserFor(identity authapi.UserIdentityInfo) (userInfo kuser.Info, err error) {
	// Retrying up to three times lets us handle race conditions with up to two conflicting identity
	// providers without returning an error
	// * A single race is possible on user creation for every conflicting identity provider
	// * A single race is possible on user creation between two instances of the same provider
	// * A single race is possible on identity creation between two instances of the same provider
	//
	// A race condition between three conflicting identity providers *and* multiple instances of the
	// same identity provider seems like a reasonable situation to return an error (you would get an
	// AlreadyExists error on either the user or the identity)
	return d.userForWithRetries(identity, 3)
}

func (d *DeterministicUserIdentityMapper) userForWithRetries(info authapi.UserIdentityInfo,
	allowedRetries int) (kuser.Info, error) {
	ctx := kapi.NewContext()

	identity, err := d.identityRegistry.GetIdentity(ctx, info.GetIdentityName())

	if kerrs.IsNotFound(err) {
		user, err := d.createIdentityAndMapping(ctx, info)
		// Only retry for AlreadyExists errors, which can occur in the following cases:
		// * The same user was created by another identity provider with the same preferred username
		// * The same user was created by another instance of this identity provider
		// * The same identity was created by another instance of this identity provider
		if kerrs.IsAlreadyExists(err) && allowedRetries > 0 {
			return d.userForWithRetries(info, allowedRetries-1)
		}
		return user, err
	}

	if err != nil {
		return nil, err
	}

	return d.getMapping(ctx, identity)
}

// createIdentityAndMapping creates an identity with a valid user reference for the given identity info
func (d *DeterministicUserIdentityMapper) createIdentityAndMapping(ctx kapi.Context,
	info authapi.UserIdentityInfo) (kuser.Info, error) {
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
	persistedUser, err := d.getOrCreateUserForIdentity(ctx, identity)
	if err != nil {
		return nil, err
	}

	// Create the identity pointing to the persistedUser
	identity.User = kapi.ObjectReference{
		Name: persistedUser.Name,
		UID:  persistedUser.UID,
	}
	if _, err := d.identityRegistry.CreateIdentity(ctx, identity); err != nil {
		return nil, err
	}

	return &kuser.DefaultInfo{
		Name:   persistedUser.Name,
		UID:    string(persistedUser.UID),
		Groups: persistedUser.Groups,
	}, nil
}

func (d *DeterministicUserIdentityMapper) getOrCreateUserForIdentity(ctx kapi.Context,
	identity *userapi.Identity) (*userapi.User, error) {

	preferredUserName := identitymapper.GetPreferredUserName(identity)

	// If an existing user references this identity, associate the identity with that user and return
	// If identity collisions are allowed, an existing user that doesn't reference this identity
	// associated with this identity and returned
	// If provisioning is allowed, a search that returns no users leads to the user being created

	// See if the user already exists
	persistedUser, err := d.userRegistry.GetUser(ctx, preferredUserName)

	if err != nil && !kerrs.IsNotFound(err) {
		// Fail on errors other than "not found"
		return nil, err
	}

	if err != nil && kerrs.IsNotFound(err) {
		if d.ProvisionUsers {
			// Try to create a user with the available name
			desiredUser := &userapi.User{
				ObjectMeta: kapi.ObjectMeta{Name: preferredUserName},
				Identities: []string{identity.Name},
			}

			// Initialize from the identity
			d.initializer.InitializeUser(identity, desiredUser)

			// Create the user
			createdUser, err := d.userRegistry.CreateUser(ctx, desiredUser)
			if err != nil {
				return nil, err
			}
			return createdUser, nil
		}
		return nil, errors.New("no user found for identity")
	}

	if util.NewStringSet(persistedUser.Identities...).Has(identity.Name) {
		// If the existing user references our identity, we're done
		return persistedUser, nil
	}

	if d.AllowIdentityCollisions {
		// If we allow identity collisions, update the persisted user's identity list and return
		persistedUser.Identities = append(persistedUser.Identities, identity.Name)
		d.userRegistry.UpdateUser(ctx, persistedUser)
		return persistedUser, nil
	}

	return nil, errors.New("no user found for identity")
}

func (d *DeterministicUserIdentityMapper) getMapping(ctx kapi.Context, identity *userapi.Identity) (kuser.Info, error) {
	if len(identity.User.Name) == 0 {
		return nil, kerrs.NewNotFound("UserIdentityMapping", identity.Name)
	}
	user, err := d.userRegistry.GetUser(ctx, identity.User.Name)
	if err != nil {
		return nil, err
	}
	if user.UID != identity.User.UID {
		glog.Errorf("identity.user.uid (%s) and user.uid (%s) do not match for identity %s",
			identity.User.UID, user.UID, identity.Name)
		return nil, kerrs.NewNotFound("UserIdentityMapping", identity.Name)
	}
	if !util.NewStringSet(user.Identities...).Has(identity.Name) {
		glog.Errorf("user.identities (%#v) does not include identity (%s)", user, identity.Name)
		return nil, kerrs.NewNotFound("UserIdentityMapping", identity.Name)
	}
	return &kuser.DefaultInfo{
		Name:   user.Name,
		UID:    string(user.UID),
		Groups: user.Groups,
	}, nil
}
