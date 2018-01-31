package useridentitymapping

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	userapi "github.com/openshift/api/user/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	userinternal "github.com/openshift/origin/pkg/user/apis/user"
)

// REST implements the RESTStorage interface in terms of an image registry and
// image repository registry. It only supports the CreateUser method and is used
// to simplify adding a new Image and tag to an ImageRepository.
type REST struct {
	userClient     userclient.UserInterface
	identityClient userclient.IdentityInterface
}

var _ rest.Getter = &REST{}
var _ rest.CreaterUpdater = &REST{}
var _ rest.Deleter = &REST{}

// NewREST returns a new REST.
func NewREST(userClient userclient.UserInterface, identityClient userclient.IdentityInterface) *REST {
	return &REST{userClient: userClient, identityClient: identityClient}
}

// New returns a new UserIdentityMapping for use with CreateUser.
func (r *REST) New() runtime.Object {
	return &userinternal.UserIdentityMapping{}
}

// GetIdentities returns the mapping for the named identity
func (s *REST) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	_, _, _, _, mapping, err := s.getRelatedObjects(ctx, name, options)
	return mapping, err
}

// CreateUser associates a user and identity if they both exist, and the identity is not already mapped to a user
func (s *REST) Create(ctx apirequest.Context, obj runtime.Object, _ rest.ValidateObjectFunc, _ bool) (runtime.Object, error) {
	mapping, ok := obj.(*userinternal.UserIdentityMapping)
	if !ok {
		return nil, kerrs.NewBadRequest("invalid type")
	}
	Strategy.PrepareForCreate(ctx, mapping)
	createdMapping, _, err := s.createOrUpdate(ctx, obj, true)
	return createdMapping, err
}

// UpdateUser associates an identity with a user.
// Both the identity and user must already exist.
// If the identity is associated with another user already, it is disassociated.
func (s *REST) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo, _ rest.ValidateObjectFunc, _ rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
	obj, err := objInfo.UpdatedObject(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	mapping, ok := obj.(*userinternal.UserIdentityMapping)
	if !ok {
		return nil, false, kerrs.NewBadRequest("invalid type")
	}
	Strategy.PrepareForUpdate(ctx, mapping, nil)
	return s.createOrUpdate(ctx, mapping, false)
}

func (s *REST) createOrUpdate(ctx apirequest.Context, obj runtime.Object, forceCreate bool) (runtime.Object, bool, error) {
	mapping := obj.(*userinternal.UserIdentityMapping)
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
		// TODO update to openshift/api
		return nil, false, kerrs.NewInvalid(userinternal.Kind("UserIdentityMapping"), mapping.Name, errs)
	}

	// GetIdentities new user
	newUser, err := s.userClient.Get(mapping.User.Name, metav1.GetOptions{})
	if kerrs.IsNotFound(err) {
		errs := field.ErrorList{field.Invalid(field.NewPath("user", "name"), mapping.User.Name, "referenced user does not exist")}
		// TODO update to openshift/api
		return nil, false, kerrs.NewInvalid(userinternal.Kind("UserIdentityMapping"), mapping.Name, errs)
	}
	if err != nil {
		return nil, false, err
	}

	// UpdateUser the new user to point at the identity. If this fails, UpdateUser is re-entrant
	if addIdentityToUser(identity, newUser) {
		if _, err := s.userClient.Update(newUser); err != nil {
			return nil, false, err
		}
	}

	// UpdateUser the identity to point at the new user. If this fails. UpdateUser is re-entrant
	if setIdentityUser(identity, newUser) {
		if updatedIdentity, err := s.identityClient.Update(identity); err != nil {
			return nil, false, err
		} else {
			identity = updatedIdentity
		}
	}

	// At this point, the mapping for the identity has been updated to the new user
	// Everything past this point is cleanup

	// UpdateUser the old user to no longer point at the identity.
	// If this fails, log the error, but continue, because UpdateUser is no longer re-entrant
	if oldUser != nil && removeIdentityFromUser(identity, oldUser) {
		if _, err := s.userClient.Update(oldUser); err != nil {
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
		if _, err := s.userClient.Update(user); err != nil {
			return nil, err
		}
	}

	// Remove the user association from the identity last.
	// If this fails, log the error, but continue, because Delete is no longer re-entrant
	// At this point, the mapping for the identity no longer exists
	if unsetIdentityUser(identity) {
		if _, err := s.identityClient.Update(identity); err != nil {
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
	mapping *userinternal.UserIdentityMapping, mappingErr error,
) {
	// Initialize errors to NotFound
	identityErr = kerrs.NewNotFound(userapi.Resource("identity"), name)
	userErr = kerrs.NewNotFound(userapi.Resource("user"), "")
	mappingErr = kerrs.NewNotFound(userapi.Resource("useridentitymapping"), name)

	// GetIdentities identity
	identity, identityErr = s.identityClient.Get(name, *options)
	if identityErr != nil {
		return
	}
	if !hasUserMapping(identity) {
		return
	}

	// GetIdentities user
	user, userErr = s.userClient.Get(identity.User.Name, *options)
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
	identity.User = corev1.ObjectReference{
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
	identity.User = corev1.ObjectReference{}
	return true
}

// mappingFor returns a UserIdentityMapping for the given user and identity
// The name and resource version of the identity mapping match the identity
func mappingFor(user *userapi.User, identity *userapi.Identity) (*userinternal.UserIdentityMapping, error) {
	return &userinternal.UserIdentityMapping{
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
