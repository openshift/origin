package identitymapper

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/types"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/test"
)

func TestGetPreferredUsername(t *testing.T) {
	identity := &api.Identity{}

	identity.ProviderUserName = "foo"
	if preferred := getPreferredUserName(identity); preferred != "foo" {
		t.Errorf("Expected %s, got %s", "foo", preferred)
	}

	identity.Extra = map[string]string{authapi.IdentityPreferredUsernameKey: "bar"}
	if preferred := getPreferredUserName(identity); preferred != "bar" {
		t.Errorf("Expected %s, got %s", "bar", preferred)
	}
}

func TestProvisionNewIdentity(t *testing.T) {
	expectedProviderName := "myprovidername"
	expectedProviderUserName := "myusername"
	expectedUserName := "myusername"
	expectedUserUID := "myuseruid"
	expectedIdentityName := "myprovidername:myusername"

	// Expect identity to fully specify a user name and uid
	expectedCreateIdentity := &api.Identity{
		ObjectMeta:       kapi.ObjectMeta{Name: expectedIdentityName},
		ProviderName:     expectedProviderName,
		ProviderUserName: expectedProviderUserName,
		User: kapi.ObjectReference{
			Name: expectedUserName,
			UID:  types.UID(expectedUserUID),
		},
		Extra: map[string]string{},
	}
	// Expect user to be populated with the right name, display name, and identity
	expectedCreateUser := &api.User{
		ObjectMeta: kapi.ObjectMeta{
			Name: expectedUserName,
		},
		Identities: []string{expectedIdentityName},
	}
	// Return a user containing a uid
	expectedCreateUserResult := &api.User{
		ObjectMeta: kapi.ObjectMeta{
			Name: expectedUserName,
			UID:  types.UID(expectedUserUID),
		},
		Identities: []string{expectedIdentityName},
	}

	expectedActions := []test.Action{
		{"GetIdentity", expectedIdentityName},
		{"GetUser", expectedProviderUserName},
		{"CreateUser", expectedCreateUser},
		{"CreateIdentity", expectedCreateIdentity},
	}

	actions := []test.Action{}
	identityRegistry := &test.IdentityRegistry{
		Create:  expectedCreateIdentity,
		Actions: &actions,
	}
	userRegistry := &test.UserRegistry{
		Create:  expectedCreateUserResult,
		Actions: &actions,
	}

	identityMapper := NewAlwaysCreateUserIdentityToUserMapper(identityRegistry, userRegistry)
	identity := authapi.NewDefaultUserIdentityInfo(expectedProviderName, expectedProviderUserName)

	identityMapper.UserFor(identity)

	for i, action := range actions {
		if len(expectedActions) <= i {
			t.Fatalf("Expected %d actions, got extras: %#v", len(expectedActions), actions[i:])
		}
		expectedAction := expectedActions[i]
		if !reflect.DeepEqual(expectedAction, action) {
			t.Fatalf("Expected\n\t%s %#v\nGot\n\t%s %#v", expectedAction.Name, expectedAction.Object, action.Name, action.Object)
		}
	}
}

func TestProvisionConflictingIdentity(t *testing.T) {
	expectedProviderName := "myprovidername"
	expectedProviderUserName := "myusername"
	expectedIdentityName := "myprovidername:myusername"
	expectedUserName := "myusername3"
	expectedUserUID := "myuseruid"

	// Expect identity to fully specify a user name and uid
	expectedCreateIdentity := &api.Identity{
		ObjectMeta:       kapi.ObjectMeta{Name: expectedIdentityName},
		ProviderName:     expectedProviderName,
		ProviderUserName: expectedProviderUserName,
		User: kapi.ObjectReference{
			Name: expectedUserName,
			UID:  types.UID(expectedUserUID),
		},
		Extra: map[string]string{},
	}
	// Expect user to be populated with the right name, display name, and identity
	expectedCreateUser := &api.User{
		ObjectMeta: kapi.ObjectMeta{
			Name: expectedUserName,
		},
		Identities: []string{expectedIdentityName},
	}
	// Return a user containing a uid
	expectedCreateUserResult := &api.User{
		ObjectMeta: kapi.ObjectMeta{
			Name: expectedUserName,
			UID:  types.UID(expectedUserUID),
		},
		Identities: []string{expectedIdentityName},
	}

	expectedActions := []test.Action{
		{"GetIdentity", expectedIdentityName},
		{"GetUser", expectedProviderUserName},
		{"GetUser", expectedProviderUserName + "2"},
		{"GetUser", expectedProviderUserName + "3"},
		{"CreateUser", expectedCreateUser},
		{"CreateIdentity", expectedCreateIdentity},
	}

	actions := []test.Action{}
	identityRegistry := &test.IdentityRegistry{
		Create:  expectedCreateIdentity,
		Actions: &actions,
	}
	userRegistry := &test.UserRegistry{
		Get: map[string]*api.User{
			expectedProviderUserName:       {},
			expectedProviderUserName + "2": {},
		},
		Create:  expectedCreateUserResult,
		Actions: &actions,
	}

	identityMapper := NewAlwaysCreateUserIdentityToUserMapper(identityRegistry, userRegistry)
	identity := authapi.NewDefaultUserIdentityInfo(expectedProviderName, expectedProviderUserName)

	identityMapper.UserFor(identity)

	for i, action := range actions {
		if len(expectedActions) <= i {
			t.Fatalf("Expected %d actions, got extras: %#v", len(expectedActions), actions[i:])
		}
		expectedAction := expectedActions[i]
		if !reflect.DeepEqual(expectedAction, action) {
			t.Fatalf("Expected\n\t%s %#v\nGot\n\t%s %#v", expectedAction.Name, expectedAction.Object, action.Name, action.Object)
		}
	}
}
