package syncgroups

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/types"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/test"
)

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

	tests := []struct {
		expectedActions []test.Action
		provisionUsers  bool
		name            string
	}{
		{
			expectedActions: []test.Action{
				{"GetIdentity", expectedIdentityName},
				{"GetUser", expectedProviderUserName},
				{"CreateUser", expectedCreateUser},
				{"CreateIdentity", expectedCreateIdentity},
			},
			provisionUsers: true,
			name:           "provisionUsers",
		},
		{
			expectedActions: []test.Action{
				{"GetIdentity", expectedIdentityName},
				{"GetUser", expectedProviderUserName},
			},
			provisionUsers: false,
			name:           "dontProvisionUsers",
		},
	}

	for _, testCase := range tests {
		actions := []test.Action{}
		identityRegistry := &test.IdentityRegistry{
			Create:  expectedCreateIdentity,
			Actions: &actions,
		}
		userRegistry := &test.UserRegistry{
			Create:  expectedCreateUserResult,
			Actions: &actions,
		}

		identityMapper := NewDeterministicUserIdentityToUserMapper(identityRegistry, userRegistry, false, testCase.provisionUsers)
		identity := authapi.NewDefaultUserIdentityInfo(expectedProviderName, expectedProviderUserName)

		identityMapper.UserFor(identity)

		for i, action := range actions {
			if len(testCase.expectedActions) <= i {
				t.Fatalf("Test %s: Expected %d actions, got extras: %#v", testCase.name, len(testCase.expectedActions), actions[i:])
			}
			expectedAction := testCase.expectedActions[i]
			if !reflect.DeepEqual(expectedAction, action) {
				t.Fatalf("Test %s: Expected\n\t%s %#v\nGot\n\t%s %#v", testCase.name, expectedAction.Name, expectedAction.Object, action.Name, action.Object)
			}
		}
	}
}

func TestConflictingIdentity(t *testing.T) {
	expectedProviderName := "myprovidername"
	expectedProviderUserName := "myusername"
	expectedIdentityName := "myprovidername:myusername"
	expectedUserName := "myusername3"
	expectedUserUID := "myuseruid"
	expectedUpdatedUserIdentities := []string{expectedIdentityName + "2", expectedIdentityName}

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
	// Expect updated user to have multiple identities
	expectedUpdatedUser := &api.User{
		ObjectMeta: kapi.ObjectMeta{
			Name: expectedUserName,
			UID:  types.UID(expectedUserUID),
		},
		Identities: expectedUpdatedUserIdentities,
	}

	tests := []struct {
		expectedActions         []test.Action
		allowIdentityCollisions bool
		name                    string
	}{
		{
			expectedActions: []test.Action{
				{"GetIdentity", expectedIdentityName},
				{"GetUser", expectedProviderUserName},
				{"CreateUser", expectedCreateUser},
				{"CreateIdentity", expectedCreateIdentity},
			},
			allowIdentityCollisions: false,
			name: "disallowIdentityCollisions",
		},
		{
			expectedActions: []test.Action{
				{"GetIdentity", expectedIdentityName},
				{"GetUser", expectedProviderUserName},
				{"UpdateUser", expectedUpdatedUser},
				{"CreateIdentity", expectedCreateIdentity},
			},
			allowIdentityCollisions: true,
			name: "allowIdentityCollisions",
		},
	}

	for _, testCase := range tests {
		actions := []test.Action{}
		identityRegistry := &test.IdentityRegistry{
			Create:  expectedCreateIdentity,
			Actions: &actions,
		}
		userRegistry := &test.UserRegistry{
			Get: map[string]*api.User{
				expectedProviderUserName: &api.User{
					ObjectMeta: kapi.ObjectMeta{
						Name: expectedUserName,
						UID:  types.UID(expectedUserUID),
					},
					Identities: []string{expectedIdentityName + "2"},
				},
			},
			Create:  expectedCreateUserResult,
			Actions: &actions,
		}

		identityMapper := NewDeterministicUserIdentityToUserMapper(identityRegistry, userRegistry, testCase.allowIdentityCollisions, false)
		identity := authapi.NewDefaultUserIdentityInfo(expectedProviderName, expectedProviderUserName)

		identityMapper.UserFor(identity)

		for i, action := range actions {
			if len(testCase.expectedActions) <= i {
				t.Fatalf("Test %s: Expected %d actions, got extras: %#v", testCase.name, len(testCase.expectedActions), actions[i:])
			}
			expectedAction := testCase.expectedActions[i]
			if !reflect.DeepEqual(expectedAction, action) {
				t.Fatalf("Test %s: Expected\n\t%s %#v\nGot\n\t%s %#v", testCase.name, expectedAction.Name, expectedAction.Object, action.Name, action.Object)
			}
		}
	}
}
