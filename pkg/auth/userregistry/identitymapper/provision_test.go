package identitymapper

import (
	"fmt"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/user/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/test"
)

type testNewIdentityGetter struct {
	called    int
	responses []interface{}
}

func (t *testNewIdentityGetter) UserForNewIdentity(ctx kapi.Context, preferredUserName string, identity *userapi.Identity) (*userapi.User, error) {
	t.called++
	if len(t.responses) < t.called {
		return nil, fmt.Errorf("Called at least %d times, only %d responses registered", t.called, len(t.responses))
	}
	switch response := t.responses[t.called-1].(type) {
	case error:
		return nil, response
	case *userapi.User:
		return response, nil
	default:
		return nil, fmt.Errorf("Invalid response type registered: %#v", response)
	}
}

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

func TestProvision(t *testing.T) {
	testcases := map[string]struct {
		ProviderName     string
		ProviderUserName string

		ExistingIdentity           *userapi.Identity
		ExistingUser               *userapi.User
		NewIdentityGetterResponses []interface{}

		ExpectedActions  []test.Action
		ExpectedError    bool
		ExpectedUserName string
	}{
		"no identity, create user succeeds": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: nil,
			ExistingUser:     nil,
			NewIdentityGetterResponses: []interface{}{
				makeUser("bobUserUID", "bob", "idp:bob"),
			},

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				// ... new identity user getter creates user
				{"CreateIdentity", makeIdentity("", "idp", "bob", "bobUserUID", "bob")},
			},
			ExpectedUserName: "bob",
		},
		"no identity, alreadyexists error retries": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: nil,
			ExistingUser:     nil,
			NewIdentityGetterResponses: []interface{}{
				kerrs.NewAlreadyExists(userapi.Resource("User"), "bob"),
				makeUser("bobUserUID", "bob", "idp:bob"),
			},

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				// ... new identity user getter returns error
				{"GetIdentity", "idp:bob"},
				// ... new identity user getter creates user
				{"CreateIdentity", makeIdentity("", "idp", "bob", "bobUserUID", "bob")},
			},
			ExpectedUserName: "bob",
		},
		"no identity, conflict error retries": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: nil,
			ExistingUser:     nil,
			NewIdentityGetterResponses: []interface{}{
				kerrs.NewConflict(userapi.Resource("User"), "bob", fmt.Errorf("conflict")),
				makeUser("bobUserUID", "bob", "idp:bob"),
			},

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				// ... new identity user getter returns error
				{"GetIdentity", "idp:bob"},
				// ... new identity user getter creates user
				{"CreateIdentity", makeIdentity("", "idp", "bob", "bobUserUID", "bob")},
			},
			ExpectedUserName: "bob",
		},
		"no identity, only retries 3 times": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: nil,
			ExistingUser:     nil,
			NewIdentityGetterResponses: []interface{}{
				kerrs.NewConflict(userapi.Resource("User"), "bob", fmt.Errorf("conflict")),
				kerrs.NewConflict(userapi.Resource("User"), "bob", fmt.Errorf("conflict")),
				kerrs.NewConflict(userapi.Resource("User"), "bob", fmt.Errorf("conflict")),
				kerrs.NewConflict(userapi.Resource("User"), "bob", fmt.Errorf("conflict")),
			},

			ExpectedActions: []test.Action{
				// original attempt
				{"GetIdentity", "idp:bob"},
				// ... new identity user getter returns error
				// retry #1
				{"GetIdentity", "idp:bob"},
				// ... new identity user getter returns error
				// retry #2
				{"GetIdentity", "idp:bob"},
				// ... new identity user getter returns error
				// retry #3
				{"GetIdentity", "idp:bob"},
				// ... new identity user getter returns error
			},
			ExpectedError: true,
		},
		"no identity, unknown error does not retry": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity: nil,
			ExistingUser:     nil,
			NewIdentityGetterResponses: []interface{}{
				fmt.Errorf("other error"),
			},

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				// ... new identity user getter returns error
			},
			ExpectedError: true,
		},

		"existing identity, no user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity:           makeIdentity("bobIdentityUID", "idp", "bob", "", ""),
			ExistingUser:               nil,
			NewIdentityGetterResponses: []interface{}{},

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
			},
			ExpectedError: true,
		},
		"existing identity, missing user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity:           makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob"),
			ExistingUser:               nil,
			NewIdentityGetterResponses: []interface{}{},

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				{"GetUser", "bob"},
			},
			ExpectedError: true,
		},
		"existing identity, invalid user UID reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity:           makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUIDInvalid", "bob"),
			ExistingUser:               makeUser("bobUserUID", "bob", "idp:bob"),
			NewIdentityGetterResponses: []interface{}{},

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				{"GetUser", "bob"},
			},
			ExpectedError: true,
		},
		"existing identity, user reference without identity backreference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity:           makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob"),
			ExistingUser:               makeUser("bobUserUID", "bob" /*, "idp:bob"*/),
			NewIdentityGetterResponses: []interface{}{},

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				{"GetUser", "bob"},
			},
			ExpectedError: true,
		},
		"existing identity, user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingIdentity:           makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob"),
			ExistingUser:               makeUser("bobUserUID", "bob", "idp:bob"),
			NewIdentityGetterResponses: []interface{}{},

			ExpectedActions: []test.Action{
				{"GetIdentity", "idp:bob"},
				{"GetUser", "bob"},
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

		newIdentityUserGetter := &testNewIdentityGetter{responses: tc.NewIdentityGetterResponses}

		provisionMapper := &provisioningIdentityMapper{
			identity:             identityRegistry,
			user:                 userRegistry,
			provisioningStrategy: newIdentityUserGetter,
		}

		identity := authapi.NewDefaultUserIdentityInfo(tc.ProviderName, tc.ProviderUserName)
		user, err := provisionMapper.UserFor(identity)
		if tc.ExpectedError != (err != nil) {
			t.Errorf("%s: Expected error=%v, got %v", k, tc.ExpectedError, err)
			continue
		}
		if !tc.ExpectedError && user.GetName() != tc.ExpectedUserName {
			t.Errorf("%s: Expected username %v, got %v", k, tc.ExpectedUserName, user.GetName())
			continue
		}

		if newIdentityUserGetter.called != len(tc.NewIdentityGetterResponses) {
			t.Errorf("%s: Expected %d calls to UserForNewIdentity, got %d", k, len(tc.NewIdentityGetterResponses), newIdentityUserGetter.called)
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
