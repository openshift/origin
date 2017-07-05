package useridentitymapping

import (
	"fmt"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/api"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
	"github.com/openshift/origin/pkg/user/registry/identity"
	"github.com/openshift/origin/pkg/user/registry/user"
)

// REST implements the RESTStorage interface in terms of an image registry and
// image repository registry. It only supports the Create method and is used
// to simplify adding a new Image and tag to an ImageRepository.
type REST struct {
	userRegistry     user.Registry
	identityRegistry identity.Registry
}

// NewREST returns a new REST.
func NewREST(userRegistry user.Registry, identityRegistry identity.Registry) *REST {
	return &REST{userRegistry: userRegistry, identityRegistry: identityRegistry}
}

// New returns a new UserIdentityMapping for use with Create.
func (r *REST) New() runtime.Object {
	return &userapi.UserIdentityMapping{}
}

// Get returns the mapping for the named identity
func (s *REST) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	_, _, _, _, mapping, err := s.getRelatedObjects(ctx, name, options)
	return mapping, err
}

// Create associates a user and identity if they both exist, and the identity is not already mapped to a user
func (s *REST) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	mapping, ok := obj.(*userapi.UserIdentityMapping)
	if !ok {
		return nil, kerrs.NewBadRequest("invalid type")
	}
	Strategy.PrepareForCreate(ctx, mapping)
	createdMapping, _, err := s.createOrUpdate(ctx, obj, true)
	return createdMapping, err
}

// Update associates an identity with a user.
// Both the identity and user must already exist.
// If the identity is associated with another user already, it is disassociated.
func (s *REST) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	obj, err := objInfo.UpdatedObject(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	mapping, ok := obj.(*userapi.UserIdentityMapping)
	if !ok {
		return nil, false, kerrs.NewBadRequest("invalid type")
	}
	Strategy.PrepareForUpdate(ctx, mapping, nil)
	return s.createOrUpdate(ctx, mapping, false)
}

func (s *REST) createOrUpdate(ctx apirequest.Context, obj runtime.Object, forceCreate bool) (runtime.Object, bool, error) {
	mapping := obj.(*userapi.UserIdentityMapping)
	identity, identityErr, oldUser, oldUserErr, oldMapping, oldMappingErr := s.getRelatedObjects(ctx, mapping.Name, &metav1.GetOptions{})

	// Ensure we didn't get any errors other than NotFound errors
	if !(oldMappingErr == nil || kerrs.IsNotFound(oldMappingErr)) {
		return nil, false, oldMappingErr
	}
	if !(identityErr == nil || kerrs.IsNotFound(identityErr)) {
		return nil, false, identityErr
	}
	if !(oldUserErr == nil || kerrs.IsNotFound(oldUserErr)) {
		return nil, false, oldUserErr
	}

	// If we expect to be creating, fail if the mapping already existed
	if forceCreate && oldMappingErr == nil {
		return nil, false, kerrs.NewAlreadyExists(userapi.Resource("useridentitymapping"), oldMapping.Name)
	}

	// Allow update to create if missing
	creating := forceCreate || kerrs.IsNotFound(oldMappingErr)
	if creating {
		// Pre-create checks with no access to oldMapping
		if err := rest.BeforeCreate(Strategy, ctx, mapping); err != nil {
			return nil, false, err
		}

		// Ensure resource version is not specified
		if len(mapping.ResourceVersion) > 0 {
			return nil, false, kerrs.NewNotFound(userapi.Resource("useridentitymapping"), mapping.Name)
		}
	} else {
		// Pre-update checks with access to oldMapping
		if err := rest.BeforeUpdate(Strategy, ctx, mapping, oldMapping); err != nil {
			return nil, false, err
		}

		// Ensure resource versions match
		if len(mapping.ResourceVersion) > 0 && mapping.ResourceVersion != oldMapping.ResourceVersion {
			return nil, false, kerrs.NewConflict(userapi.Resource("useridentitymapping"), mapping.Name, fmt.Errorf("the resource was updated to %s", oldMapping.ResourceVersion))
		}

		// If we're "updating" to the user we're already pointing to, we're already done
		if mapping.User.Name == oldMapping.User.Name {
			return oldMapping, false, nil
		}
	}

	// Validate identity
	if kerrs.IsNotFound(identityErr) {
		errs := field.ErrorList{field.Invalid(field.NewPath("identity", "name"), mapping.Identity.Name, "referenced identity does not exist")}
		return nil, false, kerrs.NewInvalid(userapi.Kind("UserIdentityMapping"), mapping.Name, errs)
	}

	// Get new user
	newUser, err := s.userRegistry.GetUser(ctx, mapping.User.Name, &metav1.GetOptions{})
	if kerrs.IsNotFound(err) {
		errs := field.ErrorList{field.Invalid(field.NewPath("user", "name"), mapping.User.Name, "referenced user does not exist")}
		return nil, false, kerrs.NewInvalid(userapi.Kind("UserIdentityMapping"), mapping.Name, errs)
	}
	if err != nil {
		return nil, false, err
	}

	// Update the new user to point at the identity. If this fails, Update is re-entrant
	if addIdentityToUser(identity, newUser) {
		if _, err := s.userRegistry.UpdateUser(ctx, newUser); err != nil {
			return nil, false, err
		}
	}

	// Update the identity to point at the new user. If this fails. Update is re-entrant
	if setIdentityUser(identity, newUser) {
		if updatedIdentity, err := s.identityRegistry.UpdateIdentity(ctx, identity); err != nil {
			return nil, false, err
		} else {
			identity = updatedIdentity
		}
	}

	// At this point, the mapping for the identity has been updated to the new user
	// Everything past this point is cleanup

	// Update the old user to no longer point at the identity.
	// If this fails, log the error, but continue, because Update is no longer re-entrant
	if oldUser != nil && removeIdentityFromUser(identity, oldUser) {
		if _, err := s.userRegistry.UpdateUser(ctx, oldUser); err != nil {
			utilruntime.HandleError(fmt.Errorf("error removing identity reference %s from user %s: %v", identity.Name, oldUser.Name, err))
		}
	}

	updatedMapping, err := mappingFor(newUser, identity)
	return updatedMapping, creating, err
}

// Delete deletes the user association for the named identity
func (s *REST) Delete(ctx apirequest.Context, name string) (runtime.Object, error) {
	identity, _, user, _, _, mappingErr := s.getRelatedObjects(ctx, name, &metav1.GetOptions{})

	if mappingErr != nil {
		return nil, mappingErr
	}

	// Disassociate the identity with the user first
	// If this fails, Delete is re-entrant
	if removeIdentityFromUser(identity, user) {
		if _, err := s.userRegistry.UpdateUser(ctx, user); err != nil {
			return nil, err
		}
	}

	// Remove the user association from the identity last.
	// If this fails, log the error, but continue, because Delete is no longer re-entrant
	// At this point, the mapping for the identity no longer exists
	if unsetIdentityUser(identity) {
		if _, err := s.identityRegistry.UpdateIdentity(ctx, identity); err != nil {
			utilruntime.HandleError(fmt.Errorf("error removing user reference %s from identity %s: %v", user.Name, identity.Name, err))
		}
	}

	return &metav1.Status{Status: metav1.StatusSuccess}, nil
}

// getRelatedObjects returns the identity, user, and mapping for the named identity
// a nil mappingErr means all objects were retrieved without errors, and correctly reference each other
func (s *REST) getRelatedObjects(ctx apirequest.Context, name string, options *metav1.GetOptions) (
	identity *userapi.Identity, identityErr error,
	user *userapi.User, userErr error,
	mapping *userapi.UserIdentityMapping, mappingErr error,
) {
	// Initialize errors to NotFound
	identityErr = kerrs.NewNotFound(userapi.Resource("identity"), name)
	userErr = kerrs.NewNotFound(userapi.Resource("user"), "")
	mappingErr = kerrs.NewNotFound(userapi.Resource("useridentitymapping"), name)

	// Get identity
	identity, identityErr = s.identityRegistry.GetIdentity(ctx, name, options)
	if identityErr != nil {
		return
	}
	if !hasUserMapping(identity) {
		return
	}

	// Get user
	user, userErr = s.userRegistry.GetUser(ctx, identity.User.Name, options)
	if userErr != nil {
		return
	}

	// Ensure relational integrity
	if !identityReferencesUser(identity, user) {
		return
	}
	if !userReferencesIdentity(user, identity) {
		return
	}

	mapping, mappingErr = mappingFor(user, identity)

	return
}

// hasUserMapping returns true if the given identity references a user
func hasUserMapping(identity *userapi.Identity) bool {
	return len(identity.User.Name) > 0
}

// identityReferencesUser returns true if the identity's user name and uid match the given user
func identityReferencesUser(identity *userapi.Identity, user *userapi.User) bool {
	return identity.User.Name == user.Name && identity.User.UID == user.UID
}

// userReferencesIdentity returns true if the user's identity list contains the given identity
func userReferencesIdentity(user *userapi.User, identity *userapi.Identity) bool {
	return sets.NewString(user.Identities...).Has(identity.Name)
}

// addIdentityToUser adds the given identity to the user's list of identities
// returns true if the user's identity list was modified
func addIdentityToUser(identity *userapi.Identity, user *userapi.User) bool {
	identities := sets.NewString(user.Identities...)
	if identities.Has(identity.Name) {
		return false
	}
	identities.Insert(identity.Name)
	user.Identities = identities.List()
	return true
}

// removeIdentityFromUser removes the given identity from the user's list of identities
// returns true if the user's identity list was modified
func removeIdentityFromUser(identity *userapi.Identity, user *userapi.User) bool {
	identities := sets.NewString(user.Identities...)
	if !identities.Has(identity.Name) {
		return false
	}
	identities.Delete(identity.Name)
	user.Identities = identities.List()
	return true
}

// setIdentityUser sets the identity to reference the given user
// returns true if the identity's user reference was modified
func setIdentityUser(identity *userapi.Identity, user *userapi.User) bool {
	if identityReferencesUser(identity, user) {
		return false
	}
	identity.User = kapi.ObjectReference{
		Name: user.Name,
		UID:  user.UID,
	}
	return true
}

// unsetIdentityUser clears the identity's user reference
// returns true if the identity's user reference was modified
func unsetIdentityUser(identity *userapi.Identity) bool {
	if !hasUserMapping(identity) {
		return false
	}
	identity.User = kapi.ObjectReference{}
	return true
}

// mappingFor returns a UserIdentityMapping for the given user and identity
// The name and resource version of the identity mapping match the identity
func mappingFor(user *userapi.User, identity *userapi.Identity) (*userapi.UserIdentityMapping, error) {
	return &userapi.UserIdentityMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name:            identity.Name,
			ResourceVersion: identity.ResourceVersion,
			UID:             identity.UID,
		},
		Identity: kapi.ObjectReference{
			Name: identity.Name,
			UID:  identity.UID,
		},
		User: kapi.ObjectReference{
			Name: user.Name,
			UID:  user.UID,
		},
	}, nil
}
