package identitymapper

import (
	"reflect"
	"testing"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/user/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/test"
	mappingregistry "github.com/openshift/origin/pkg/user/registry/useridentitymapping"
)

func TestLookup(t *testing.T) {
	testcases := map[string]struct {
		ProviderName     string
		ProviderUserName string

		ExistingIdentity *userapi.Identity
		ExistingUser     *userapi.User

		ExpectedActions  []test.Action
		ExpectedError    bool
		ExpectedUserName string
	}{
		"no identity": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: nil,
			ExistingUser:     nil,

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
			},
			ExpectedError: true,
		},

		"existing identity, no user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: makeIdentity("bobIdentityUID", "idp", "bob", "", ""),
			ExistingUser:     nil,

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
			},
			ExpectedError: true,
		},
		"existing identity, missing user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob"),
			ExistingUser:     nil,

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				{"GetUser", "bob"},
			},
			ExpectedError: true,
		},
		"existing identity, invalid user UID reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUIDInvalid", "bob"),
			ExistingUser:     makeUser("bobUserUID", "bob", "idp:bob"),

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				{"GetUser", "bob"},
			},
			ExpectedError: true,
		},
		"existing identity, user reference without identity backreference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob"),
			ExistingUser:     makeUser("bobUserUID", "bob" /*, "idp:bob"*/),

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				{"GetUser", "bob"},
			},
			ExpectedError: true,
		},
		"existing identity, user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob"),
			ExistingUser:     makeUser("bobUserUID", "bob", "idp:bob"),

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				{"GetUser", "bob"},
				{"GetUser", "bob"}, // extra request is for group lookup
			},
			ExpectedUserName: "bob",
		},
	}

	for k, tc := range testcases {
		actions := []test.Action{}
		identityRegistry := &test.IdentityRegistry{
			Get:     map[string]*api.Identity{},
			Actions: &actions,
		}
		userRegistry := &test.UserRegistry{
			Get:     map[string]*api.User{},
			Actions: &actions,
		}
		if tc.ExistingIdentity != nil {
			identityRegistry.Get[tc.ExistingIdentity.Name] = tc.ExistingIdentity
		}
		if tc.ExistingUser != nil {
			userRegistry.Get[tc.ExistingUser.Name] = tc.ExistingUser
		}

		mappingStorage := mappingregistry.NewREST(userRegistry, identityRegistry)
		mappingRegistry := mappingregistry.NewRegistry(mappingStorage)

		lookupMapper := &lookupIdentityMapper{
			mappings: mappingRegistry,
			users:    userRegistry,
		}

		identity := authapi.NewDefaultUserIdentityInfo(tc.ProviderName, tc.ProviderUserName)
		user, err := lookupMapper.UserFor(identity)
		if tc.ExpectedError != (err != nil) {
			t.Errorf("%s: Expected error=%v, got %v", k, tc.ExpectedError, err)
			continue
		}
		if !tc.ExpectedError && user.GetName() != tc.ExpectedUserName {
			t.Errorf("%s: Expected username %v, got %v", k, tc.ExpectedUserName, user.GetName())
			continue
		}

		for i, action := range actions {
			if len(tc.ExpectedActions) <= i {
				t.Fatalf("%s: expected %d actions, got extras: %#v", k, len(tc.ExpectedActions), actions[i:])
				continue
			}
			expectedAction := tc.ExpectedActions[i]
			if !reflect.DeepEqual(expectedAction, action) {
				t.Fatalf("%s: expected\n\t%s %#v\nGot\n\t%s %#v", k, expectedAction.Name, expectedAction.Object, action.Name, action.Object)
				continue
			}
		}
		if len(actions) < len(tc.ExpectedActions) {
			t.Errorf("Missing %d additional actions:\n\t%#v", len(tc.ExpectedActions)-len(actions), tc.ExpectedActions[len(actions):])
		}
	}
}
