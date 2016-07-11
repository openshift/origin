package useridentitymapping

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	kapirest "k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/test"

	_ "github.com/openshift/origin/pkg/api/install"
)

var sequence = 0

func makeUser() *api.User {
	sequence++
	return makeUserFromSequence(sequence)
}

func makeUserFromSequence(sequence int) *api.User {
	userName := fmt.Sprintf("myuser-%d", sequence)
	userUID := types.UID(fmt.Sprintf("useruid-%d", sequence))
	userResourceVersion := fmt.Sprintf("%d", sequence+100)

	return &api.User{
		ObjectMeta: kapi.ObjectMeta{Name: userName, UID: userUID, ResourceVersion: userResourceVersion},
	}
}

func makeIdentity() *api.Identity {
	sequence++
	return makeIdentityFromSequence(sequence)
}

func makeIdentityFromSequence(sequence int) *api.Identity {
	providerName := fmt.Sprintf("providername-%d", sequence)
	providerUserName := fmt.Sprintf("providerusername-%d", sequence)
	identityName := fmt.Sprintf("%s:%s", providerName, providerUserName)
	identityUID := types.UID(fmt.Sprintf("identityuid-%d", sequence))
	identityResourceVersion := fmt.Sprintf("%d", sequence+200)

	return &api.Identity{
		ObjectMeta:       kapi.ObjectMeta{Name: identityName, UID: identityUID, ResourceVersion: identityResourceVersion},
		ProviderName:     providerName,
		ProviderUserName: providerUserName,
	}
}

func makeAssociated() (*api.User, *api.Identity) {
	sequence++
	return associate(makeUserFromSequence(sequence), makeIdentityFromSequence(sequence))
}

func makeUnassociated() (*api.User, *api.Identity) {
	sequence++
	return makeUserFromSequence(sequence), makeIdentityFromSequence(sequence)
}

func associate(user *api.User, identity *api.Identity) (*api.User, *api.Identity) {
	userCopy := *user
	identityCopy := *identity
	addIdentityToUser(&identityCopy, &userCopy)
	setIdentityUser(&identityCopy, &userCopy)
	return &userCopy, &identityCopy
}

func disassociate(user *api.User, identity *api.Identity) (*api.User, *api.Identity) {
	userCopy := *user
	identityCopy := *identity
	removeIdentityFromUser(&identityCopy, &userCopy)
	unsetIdentityUser(&identityCopy)
	return &userCopy, &identityCopy
}

func setupRegistries(identity *api.Identity, users ...*api.User) (*[]test.Action, *test.UserRegistry, *test.IdentityRegistry, *REST) {
	actions := &[]test.Action{}

	userRegistry := &test.UserRegistry{
		Get:       map[string]*api.User{},
		GetErr:    map[string]error{},
		UpdateErr: map[string]error{},
		Actions:   actions,
	}
	for _, user := range users {
		userRegistry.Get[user.Name] = user
	}

	identityRegistry := &test.IdentityRegistry{
		Get:     map[string]*api.Identity{},
		GetErr:  map[string]error{},
		Actions: actions,
	}
	if identity != nil {
		identityRegistry.Get[identity.Name] = identity
	}

	rest := NewREST(userRegistry, identityRegistry)

	return actions, userRegistry, identityRegistry, rest
}

func verifyActions(expectedActions []test.Action, actualActions []test.Action, t *testing.T) {
	for i, actualAction := range actualActions {
		if len(expectedActions) <= i {
			t.Errorf("Expected %d actions, got extras: %#v", len(expectedActions), actualActions[i:])
			break
		}
		expectedAction := expectedActions[i]
		if !reflect.DeepEqual(expectedAction, actualAction) {
			t.Errorf("Expected\n\t%s %#v\nGot\n\t%s %#v", expectedAction.Name, expectedAction.Object, actualAction.Name, actualAction.Object)
		}
	}
	if len(expectedActions) > len(actualActions) {
		t.Errorf("Expected %d actions, missing: %#v", len(expectedActions), expectedActions[len(actualActions):])
	}
}

func verifyMapping(object runtime.Object, user *api.User, identity *api.Identity, t *testing.T) {
	mapping, ok := object.(*api.UserIdentityMapping)
	if !ok {
		t.Errorf("Expected mapping, got %#v", object)
		return
	}
	if mapping.User.Name != user.Name {
		t.Errorf("Expected user.name %s, got %s", user.Name, mapping.User.Name)
	}
	if mapping.User.UID != user.UID {
		t.Errorf("Expected user.uid %s, got %s", user.UID, mapping.User.UID)
	}
	if mapping.Identity.Name != identity.Name {
		t.Errorf("Expected identity.name %s, got %s", identity.Name, mapping.Identity.Name)
	}
}

func TestGet(t *testing.T) {
	user, identity := makeAssociated()
	expectedActions := []test.Action{
		{"GetIdentity", identity.Name},
		{"GetUser", user.Name},
	}

	actions, _, _, rest := setupRegistries(identity, user)
	mapping, err := rest.Get(kapi.NewContext(), identity.Name)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
	verifyMapping(mapping, user, identity, t)
}

func TestGetMissingIdentity(t *testing.T) {
	user, identity := makeAssociated()
	expectedActions := []test.Action{
		{"GetIdentity", identity.Name},
	}

	actions, _, _, rest := setupRegistries(nil, user)
	_, err := rest.Get(kapi.NewContext(), identity.Name)

	if err == nil {
		t.Errorf("Expected error, got none")
	}
	verifyActions(expectedActions, *actions, t)
}

func TestGetIdentityWithoutUser(t *testing.T) {
	identity := makeIdentity()
	expectedActions := []test.Action{
		{"GetIdentity", identity.Name},
	}

	actions, _, _, rest := setupRegistries(identity)
	_, err := rest.Get(kapi.NewContext(), identity.Name)

	if err == nil {
		t.Errorf("Expected error, got none")
	}
	if !kerrs.IsNotFound(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestGetMissingUser(t *testing.T) {
	user, identity := makeAssociated()
	expectedActions := []test.Action{
		{"GetIdentity", identity.Name},
		{"GetUser", user.Name},
	}

	actions, _, _, rest := setupRegistries(identity)
	_, err := rest.Get(kapi.NewContext(), identity.Name)

	if err == nil {
		t.Errorf("Expected error, got none")
	}
	if !kerrs.IsNotFound(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestGetUserWithoutIdentity(t *testing.T) {
	user, identity := makeAssociated()
	user.Identities = []string{}
	expectedActions := []test.Action{
		{"GetIdentity", identity.Name},
		{"GetUser", user.Name},
	}

	actions, _, _, rest := setupRegistries(identity, user)
	_, err := rest.Get(kapi.NewContext(), identity.Name)

	if err == nil {
		t.Errorf("Expected error, got none")
	}
	if !kerrs.IsNotFound(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestCreate(t *testing.T) {
	associatedUser, associatedIdentity := makeAssociated()
	unassociatedUser, unassociatedIdentity := disassociate(associatedUser, associatedIdentity)
	expectedActions := []test.Action{
		{"GetIdentity", unassociatedIdentity.Name},
		{"GetUser", unassociatedUser.Name},
		{"UpdateUser", associatedUser},
		{"UpdateIdentity", associatedIdentity},
	}

	mapping := &api.UserIdentityMapping{
		Identity: kapi.ObjectReference{Name: unassociatedIdentity.Name},
		User:     kapi.ObjectReference{Name: unassociatedUser.Name},
	}

	actions, _, _, rest := setupRegistries(unassociatedIdentity, unassociatedUser)
	createdMapping, err := rest.Create(kapi.NewContext(), mapping)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
	verifyMapping(createdMapping, associatedUser, associatedIdentity, t)
}

func TestCreateExists(t *testing.T) {
	user, identity := makeAssociated()
	expectedActions := []test.Action{
		{"GetIdentity", identity.Name},
		{"GetUser", user.Name},
	}

	mapping := &api.UserIdentityMapping{
		Identity: kapi.ObjectReference{Name: identity.Name},
		User:     kapi.ObjectReference{Name: user.Name},
	}

	actions, _, _, rest := setupRegistries(identity, user)
	_, err := rest.Create(kapi.NewContext(), mapping)

	if err == nil {
		t.Errorf("Expected error, got none")
	}
	if !kerrs.IsAlreadyExists(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestCreateMissingIdentity(t *testing.T) {
	user, identity := makeUnassociated()
	expectedActions := []test.Action{
		{"GetIdentity", identity.Name},
	}

	mapping := &api.UserIdentityMapping{
		Identity: kapi.ObjectReference{Name: identity.Name},
		User:     kapi.ObjectReference{Name: user.Name},
	}

	actions, _, _, rest := setupRegistries(nil, user)
	_, err := rest.Create(kapi.NewContext(), mapping)

	if err == nil {
		t.Errorf("Expected error, got none")
	}
	if !kerrs.IsInvalid(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestCreateMissingUser(t *testing.T) {
	user, identity := makeUnassociated()
	expectedActions := []test.Action{
		{"GetIdentity", identity.Name},
		{"GetUser", user.Name},
	}

	mapping := &api.UserIdentityMapping{
		Identity: kapi.ObjectReference{Name: identity.Name},
		User:     kapi.ObjectReference{Name: user.Name},
	}

	actions, _, _, rest := setupRegistries(identity)
	_, err := rest.Create(kapi.NewContext(), mapping)

	if err == nil {
		t.Errorf("Expected error, got none")
	}
	if !kerrs.IsInvalid(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestCreateUserUpdateError(t *testing.T) {
	associatedUser, associatedIdentity := makeAssociated()
	unassociatedUser, unassociatedIdentity := disassociate(associatedUser, associatedIdentity)
	expectedActions := []test.Action{
		{"GetIdentity", unassociatedIdentity.Name},
		{"GetUser", unassociatedUser.Name},
		{"UpdateUser", associatedUser},
	}
	expectedErr := errors.New("Update error")

	mapping := &api.UserIdentityMapping{
		Identity: kapi.ObjectReference{Name: unassociatedIdentity.Name},
		User:     kapi.ObjectReference{Name: unassociatedUser.Name},
	}

	actions, userRegistry, _, rest := setupRegistries(unassociatedIdentity, unassociatedUser)
	userRegistry.UpdateErr[associatedUser.Name] = expectedErr
	_, err := rest.Create(kapi.NewContext(), mapping)

	if err == nil {
		t.Errorf("Expected error, got none")
	}
	if err != expectedErr {
		t.Errorf("Unexpected error: %#v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestCreateIdentityUpdateError(t *testing.T) {
	associatedUser, associatedIdentity := makeAssociated()
	unassociatedUser, unassociatedIdentity := disassociate(associatedUser, associatedIdentity)
	expectedActions := []test.Action{
		{"GetIdentity", unassociatedIdentity.Name},
		{"GetUser", unassociatedUser.Name},
		{"UpdateUser", associatedUser},
		{"UpdateIdentity", associatedIdentity},
	}

	mapping := &api.UserIdentityMapping{
		Identity: kapi.ObjectReference{Name: unassociatedIdentity.Name},
		User:     kapi.ObjectReference{Name: unassociatedUser.Name},
	}

	actions, _, identityRegistry, rest := setupRegistries(unassociatedIdentity, unassociatedUser)
	identityRegistry.UpdateErr = errors.New("Update error")
	_, err := rest.Create(kapi.NewContext(), mapping)

	if err == nil {
		t.Errorf("Expected error, got none")
	}
	if err != identityRegistry.UpdateErr {
		t.Errorf("Unexpected error: %#v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestUpdate(t *testing.T) {
	// Starting conditions
	associatedUser1, associatedIdentity1User1 := makeAssociated()
	unassociatedUser2 := makeUser()
	// Finishing conditions
	unassociatedUser1, unassociatedIdentity1 := disassociate(associatedUser1, associatedIdentity1User1)
	associatedUser2, associatedIdentity1User2 := associate(unassociatedUser2, unassociatedIdentity1)

	expectedActions := []test.Action{
		// Existing mapping lookup
		{"GetIdentity", associatedIdentity1User1.Name},
		{"GetUser", associatedUser1.Name},
		// New user lookup
		{"GetUser", unassociatedUser2.Name},
		// New user update
		{"UpdateUser", associatedUser2},
		// Identity update
		{"UpdateIdentity", associatedIdentity1User2},
		// Old user cleanup
		{"UpdateUser", unassociatedUser1},
	}

	mapping := &api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{ResourceVersion: unassociatedIdentity1.ResourceVersion},
		Identity:   kapi.ObjectReference{Name: unassociatedIdentity1.Name},
		User:       kapi.ObjectReference{Name: unassociatedUser2.Name},
	}

	actions, _, _, rest := setupRegistries(associatedIdentity1User1, associatedUser1, unassociatedUser2)
	createdMapping, created, err := rest.Update(kapi.NewContext(), mapping.Name, kapirest.DefaultUpdatedObjectInfo(mapping, kapi.Scheme))

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if created {
		t.Errorf("Unexpected created")
	}
	verifyActions(expectedActions, *actions, t)
	verifyMapping(createdMapping, associatedUser2, associatedIdentity1User2, t)
}

func TestUpdateMissingIdentity(t *testing.T) {
	// Starting conditions
	associatedUser1, associatedIdentity1User1 := makeAssociated()
	unassociatedUser2 := makeUser()
	// Finishing conditions
	_, unassociatedIdentity1 := disassociate(associatedUser1, associatedIdentity1User1)

	expectedActions := []test.Action{
		// Existing mapping lookup
		{"GetIdentity", associatedIdentity1User1.Name},
	}

	mapping := &api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{ResourceVersion: unassociatedIdentity1.ResourceVersion},
		Identity:   kapi.ObjectReference{Name: unassociatedIdentity1.Name},
		User:       kapi.ObjectReference{Name: unassociatedUser2.Name},
	}

	actions, _, _, rest := setupRegistries(nil, associatedUser1, unassociatedUser2)
	_, _, err := rest.Update(kapi.NewContext(), mapping.Name, kapirest.DefaultUpdatedObjectInfo(mapping, kapi.Scheme))

	if err == nil {
		t.Errorf("Expected error: %v", err)
	}
	if !kerrs.IsInvalid(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestUpdateMissingUser(t *testing.T) {
	// Starting conditions
	associatedUser1, associatedIdentity1User1 := makeAssociated()
	unassociatedUser2 := makeUser()
	// Finishing conditions
	_, unassociatedIdentity1 := disassociate(associatedUser1, associatedIdentity1User1)

	expectedActions := []test.Action{
		// Existing mapping lookup
		{"GetIdentity", associatedIdentity1User1.Name},
		{"GetUser", associatedUser1.Name},
		// New user lookup
		{"GetUser", unassociatedUser2.Name},
	}

	mapping := &api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{ResourceVersion: unassociatedIdentity1.ResourceVersion},
		Identity:   kapi.ObjectReference{Name: unassociatedIdentity1.Name},
		User:       kapi.ObjectReference{Name: unassociatedUser2.Name},
	}

	actions, _, _, rest := setupRegistries(associatedIdentity1User1, associatedUser1)
	_, _, err := rest.Update(kapi.NewContext(), mapping.Name, kapirest.DefaultUpdatedObjectInfo(mapping, kapi.Scheme))

	if err == nil {
		t.Errorf("Expected error: %v", err)
	}
	if !kerrs.IsInvalid(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestUpdateOldUserMatches(t *testing.T) {
	user, identity := makeAssociated()

	expectedActions := []test.Action{
		// Existing mapping lookup
		{"GetIdentity", identity.Name},
		{"GetUser", user.Name},
	}

	mapping := &api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{ResourceVersion: identity.ResourceVersion},
		Identity:   kapi.ObjectReference{Name: identity.Name},
		User:       kapi.ObjectReference{Name: user.Name},
	}

	actions, _, _, rest := setupRegistries(identity, user)
	createdMapping, created, err := rest.Update(kapi.NewContext(), mapping.Name, kapirest.DefaultUpdatedObjectInfo(mapping, kapi.Scheme))

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if created {
		t.Errorf("Unexpected created")
	}
	verifyActions(expectedActions, *actions, t)
	verifyMapping(createdMapping, user, identity, t)
}

func TestUpdateWithEmptyResourceVersion(t *testing.T) {
	// Starting conditions
	associatedUser1, associatedIdentity1User1 := makeAssociated()
	unassociatedUser2 := makeUser()
	// Finishing conditions
	_, unassociatedIdentity1 := disassociate(associatedUser1, associatedIdentity1User1)

	expectedActions := []test.Action{
		// Existing mapping lookup
		{"GetIdentity", associatedIdentity1User1.Name},
		{"GetUser", associatedUser1.Name},
	}

	mapping := &api.UserIdentityMapping{
		Identity: kapi.ObjectReference{Name: unassociatedIdentity1.Name},
		User:     kapi.ObjectReference{Name: unassociatedUser2.Name},
	}

	actions, _, _, rest := setupRegistries(associatedIdentity1User1, associatedUser1, unassociatedUser2)
	_, _, err := rest.Update(kapi.NewContext(), mapping.Name, kapirest.DefaultUpdatedObjectInfo(mapping, kapi.Scheme))

	if err == nil {
		t.Errorf("Expected error")
	}
	if !kerrs.IsInvalid(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestUpdateWithMismatchedResourceVersion(t *testing.T) {
	// Starting conditions
	associatedUser1, associatedIdentity1User1 := makeAssociated()
	unassociatedUser2 := makeUser()
	// Finishing conditions
	_, unassociatedIdentity1 := disassociate(associatedUser1, associatedIdentity1User1)

	expectedActions := []test.Action{
		// Existing mapping lookup
		{"GetIdentity", associatedIdentity1User1.Name},
		{"GetUser", associatedUser1.Name},
	}

	mapping := &api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{ResourceVersion: "123"},
		Identity:   kapi.ObjectReference{Name: unassociatedIdentity1.Name},
		User:       kapi.ObjectReference{Name: unassociatedUser2.Name},
	}

	actions, _, _, rest := setupRegistries(associatedIdentity1User1, associatedUser1, unassociatedUser2)
	_, _, err := rest.Update(kapi.NewContext(), mapping.Name, kapirest.DefaultUpdatedObjectInfo(mapping, kapi.Scheme))

	if err == nil {
		t.Errorf("Expected error")
	}
	if !kerrs.IsConflict(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestUpdateOldUserUpdateError(t *testing.T) {
	// Starting conditions
	associatedUser1, associatedIdentity1User1 := makeAssociated()
	unassociatedUser2 := makeUser()
	// Finishing conditions
	unassociatedUser1, unassociatedIdentity1 := disassociate(associatedUser1, associatedIdentity1User1)
	associatedUser2, associatedIdentity1User2 := associate(unassociatedUser2, unassociatedIdentity1)

	expectedActions := []test.Action{
		// Existing mapping lookup
		{"GetIdentity", associatedIdentity1User1.Name},
		{"GetUser", associatedUser1.Name},
		// New user lookup
		{"GetUser", unassociatedUser2.Name},
		// New user update
		{"UpdateUser", associatedUser2},
		// Identity update
		{"UpdateIdentity", associatedIdentity1User2},
		// Old user cleanup
		{"UpdateUser", unassociatedUser1},
	}
	expectedErr := errors.New("Couldn't update old user")

	mapping := &api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{ResourceVersion: unassociatedIdentity1.ResourceVersion},
		Identity:   kapi.ObjectReference{Name: unassociatedIdentity1.Name},
		User:       kapi.ObjectReference{Name: unassociatedUser2.Name},
	}

	actions, userRegistry, _, rest := setupRegistries(associatedIdentity1User1, associatedUser1, unassociatedUser2)
	userRegistry.UpdateErr[unassociatedUser1.Name] = expectedErr
	createdMapping, created, err := rest.Update(kapi.NewContext(), mapping.Name, kapirest.DefaultUpdatedObjectInfo(mapping, kapi.Scheme))

	// An error cleaning up the old user shouldn't manifest as an update failure, since the mapping was successfully updated
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if created {
		t.Errorf("Unexpected created")
	}
	verifyActions(expectedActions, *actions, t)
	verifyMapping(createdMapping, associatedUser2, associatedIdentity1User2, t)
}

func TestUpdateUserUpdateError(t *testing.T) {
	// Starting conditions
	associatedUser1, associatedIdentity1User1 := makeAssociated()
	unassociatedUser2 := makeUser()
	// Finishing conditions
	_, unassociatedIdentity1 := disassociate(associatedUser1, associatedIdentity1User1)
	associatedUser2, _ := associate(unassociatedUser2, unassociatedIdentity1)

	expectedActions := []test.Action{
		// Existing mapping lookup
		{"GetIdentity", associatedIdentity1User1.Name},
		{"GetUser", associatedUser1.Name},
		// New user lookup
		{"GetUser", unassociatedUser2.Name},
		// New user update
		{"UpdateUser", associatedUser2},
	}
	expectedErr := errors.New("Couldn't update new user")

	mapping := &api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{ResourceVersion: unassociatedIdentity1.ResourceVersion},
		Identity:   kapi.ObjectReference{Name: unassociatedIdentity1.Name},
		User:       kapi.ObjectReference{Name: unassociatedUser2.Name},
	}

	actions, userRegistry, _, rest := setupRegistries(associatedIdentity1User1, associatedUser1, unassociatedUser2)
	userRegistry.UpdateErr[associatedUser2.Name] = expectedErr
	_, _, err := rest.Update(kapi.NewContext(), mapping.Name, kapirest.DefaultUpdatedObjectInfo(mapping, kapi.Scheme))

	if err == nil {
		t.Errorf("Expected error")
	}
	if err != expectedErr {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestUpdateIdentityUpdateError(t *testing.T) {
	// Starting conditions
	associatedUser1, associatedIdentity1User1 := makeAssociated()
	unassociatedUser2 := makeUser()
	// Finishing conditions
	_, unassociatedIdentity1 := disassociate(associatedUser1, associatedIdentity1User1)
	associatedUser2, associatedIdentity1User2 := associate(unassociatedUser2, unassociatedIdentity1)

	expectedActions := []test.Action{
		// Existing mapping lookup
		{"GetIdentity", associatedIdentity1User1.Name},
		{"GetUser", associatedUser1.Name},
		// New user lookup
		{"GetUser", unassociatedUser2.Name},
		// New user update
		{"UpdateUser", associatedUser2},
		// Identity update
		{"UpdateIdentity", associatedIdentity1User2},
	}
	expectedErr := errors.New("Couldn't update identity")

	mapping := &api.UserIdentityMapping{
		ObjectMeta: kapi.ObjectMeta{ResourceVersion: unassociatedIdentity1.ResourceVersion},
		Identity:   kapi.ObjectReference{Name: unassociatedIdentity1.Name},
		User:       kapi.ObjectReference{Name: unassociatedUser2.Name},
	}

	actions, _, identityRegistry, rest := setupRegistries(associatedIdentity1User1, associatedUser1, unassociatedUser2)
	identityRegistry.UpdateErr = expectedErr
	_, _, err := rest.Update(kapi.NewContext(), mapping.Name, kapirest.DefaultUpdatedObjectInfo(mapping, kapi.Scheme))

	if err == nil {
		t.Errorf("Expected error")
	}
	if err != expectedErr {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestDelete(t *testing.T) {
	associatedUser, associatedIdentity := makeAssociated()
	unassociatedUser, unassociatedIdentity := disassociate(associatedUser, associatedIdentity)
	expectedActions := []test.Action{
		{"GetIdentity", associatedIdentity.Name},
		{"GetUser", associatedUser.Name},
		{"UpdateUser", unassociatedUser},
		{"UpdateIdentity", unassociatedIdentity},
	}

	actions, _, _, rest := setupRegistries(associatedIdentity, associatedUser)
	_, err := rest.Delete(kapi.NewContext(), associatedIdentity.Name)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestDeleteMissingIdentity(t *testing.T) {
	associatedUser, associatedIdentity := makeAssociated()
	expectedActions := []test.Action{
		{"GetIdentity", associatedIdentity.Name},
	}

	actions, _, _, rest := setupRegistries(nil, associatedUser)
	_, err := rest.Delete(kapi.NewContext(), associatedIdentity.Name)

	if err == nil {
		t.Errorf("Expected error")
	}
	if !kerrs.IsNotFound(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestDeleteMissingUser(t *testing.T) {
	associatedUser, associatedIdentity := makeAssociated()
	expectedActions := []test.Action{
		{"GetIdentity", associatedIdentity.Name},
		{"GetUser", associatedUser.Name},
	}

	actions, _, _, rest := setupRegistries(associatedIdentity)
	_, err := rest.Delete(kapi.NewContext(), associatedIdentity.Name)

	if err == nil {
		t.Errorf("Expected error")
	}
	if !kerrs.IsNotFound(err) {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestDeleteUserUpdateError(t *testing.T) {
	associatedUser, associatedIdentity := makeAssociated()
	unassociatedUser, _ := disassociate(associatedUser, associatedIdentity)
	expectedActions := []test.Action{
		{"GetIdentity", associatedIdentity.Name},
		{"GetUser", associatedUser.Name},
		{"UpdateUser", unassociatedUser},
	}
	expectedErr := errors.New("Cannot update user")

	actions, userRegistry, _, rest := setupRegistries(associatedIdentity, associatedUser)
	userRegistry.UpdateErr[associatedUser.Name] = expectedErr
	_, err := rest.Delete(kapi.NewContext(), associatedIdentity.Name)

	if err == nil {
		t.Errorf("Expected error")
	}
	if err != expectedErr {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}

func TestDeleteIdentityUpdateError(t *testing.T) {
	associatedUser, associatedIdentity := makeAssociated()
	unassociatedUser, unassociatedIdentity := disassociate(associatedUser, associatedIdentity)
	expectedActions := []test.Action{
		{"GetIdentity", associatedIdentity.Name},
		{"GetUser", associatedUser.Name},
		{"UpdateUser", unassociatedUser},
		{"UpdateIdentity", unassociatedIdentity},
	}
	expectedErr := errors.New("Cannot update identity")

	actions, _, identityRegistry, rest := setupRegistries(associatedIdentity, associatedUser)
	identityRegistry.UpdateErr = expectedErr
	_, err := rest.Delete(kapi.NewContext(), associatedIdentity.Name)

	// An error cleaning up the identity reference shouldn't manifest as an update failure, since the mapping no longer exists
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	verifyActions(expectedActions, *actions, t)
}
