package identitymapper

import (
	"context"
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"

	"k8s.io/apimachinery/pkg/api/equality"
	kerrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	clienttesting "k8s.io/client-go/testing"

	userapi "github.com/openshift/api/user/v1"
	userv1fakeclient "github.com/openshift/client-go/user/clientset/versioned/fake"
	authapi "github.com/openshift/oauth-server/pkg/api"
)

type testNewIdentityGetter struct {
	called    int
	responses []interface{}
}

func (t *testNewIdentityGetter) UserForNewIdentity(ctx context.Context, preferredUserName string, identity *userapi.Identity) (*userapi.User, error) {
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
	identity := &userapi.Identity{}

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

		ExistingObjects            []runtime.Object
		NewIdentityGetterResponses []interface{}

		ValidateActions  func(t *testing.T, actions []clienttesting.Action)
		ExpectedError    bool
		ExpectedUserName string
	}{
		"no identity, create user succeeds": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			NewIdentityGetterResponses: []interface{}{
				makeUser("bobUserUID", "bob", "idp:bob"),
			},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "identities") || actions[0].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				// ... new identity user getter creates user
				if !actions[1].Matches("create", "identities") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[1].(clienttesting.CreateAction).GetObject().(*userapi.Identity)
				if expected := makeIdentity("", "idp", "bob", "bobUserUID", "bob"); !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
			ExpectedUserName: "bob",
		},
		"no identity, alreadyexists error retries": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			NewIdentityGetterResponses: []interface{}{
				kerrs.NewAlreadyExists(userapi.Resource("User"), "bob"),
				makeUser("bobUserUID", "bob", "idp:bob"),
			},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 3 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "identities") || actions[0].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				// ... new identity user getter returns error
				if !actions[1].Matches("get", "identities") || actions[1].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				// ... new identity user getter creates user
				if !actions[2].Matches("create", "identities") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[2].(clienttesting.CreateAction).GetObject().(*userapi.Identity)
				if expected := makeIdentity("", "idp", "bob", "bobUserUID", "bob"); !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
			ExpectedUserName: "bob",
		},
		"no identity, conflict error retries": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			NewIdentityGetterResponses: []interface{}{
				kerrs.NewConflict(userapi.Resource("User"), "bob", fmt.Errorf("conflict")),
				makeUser("bobUserUID", "bob", "idp:bob"),
			},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 3 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "identities") || actions[0].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				// ... new identity user getter returns error
				if !actions[1].Matches("get", "identities") || actions[1].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				// ... new identity user getter creates user
				if !actions[2].Matches("create", "identities") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[2].(clienttesting.CreateAction).GetObject().(*userapi.Identity)
				if expected := makeIdentity("", "idp", "bob", "bobUserUID", "bob"); !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
			ExpectedUserName: "bob",
		},
		"no identity, only retries 3 times": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			NewIdentityGetterResponses: []interface{}{
				kerrs.NewConflict(userapi.Resource("User"), "bob", fmt.Errorf("conflict")),
				kerrs.NewConflict(userapi.Resource("User"), "bob", fmt.Errorf("conflict")),
				kerrs.NewConflict(userapi.Resource("User"), "bob", fmt.Errorf("conflict")),
				kerrs.NewConflict(userapi.Resource("User"), "bob", fmt.Errorf("conflict")),
			},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 4 {
					t.Fatal(spew.Sdump(actions))
				}
				// original attempt
				if !actions[0].Matches("get", "identities") || actions[0].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				// ... new identity user getter returns error
				// retry #1
				if !actions[1].Matches("get", "identities") || actions[1].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				// ... new identity user getter returns error
				// retry #2
				if !actions[2].Matches("get", "identities") || actions[2].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				// ... new identity user getter returns error
				// retry #3
				if !actions[3].Matches("get", "identities") || actions[3].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				// ... new identity user getter returns error
			},

			ExpectedError: true,
		},
		"no identity, unknown error does not retry": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			NewIdentityGetterResponses: []interface{}{
				fmt.Errorf("other error"),
			},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "identities") || actions[0].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				// ... new identity user getter returns error
			},

			ExpectedError: true,
		},

		"existing identity, no user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingObjects:            []runtime.Object{makeIdentity("bobIdentityUID", "idp", "bob", "", "")},
			NewIdentityGetterResponses: []interface{}{},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "identities") || actions[0].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
			},
			ExpectedError: true,
		},
		"existing identity, missing user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingObjects:            []runtime.Object{makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob")},
			NewIdentityGetterResponses: []interface{}{},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "identities") || actions[0].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("get", "users") || actions[1].(clienttesting.GetAction).GetName() != "bob" {
					t.Error(spew.Sdump(actions))
				}
			},
			ExpectedError: true,
		},
		"existing identity, invalid user UID reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingObjects: []runtime.Object{
				makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUIDInvalid", "bob"),
				makeUser("bobUserUID", "bob", "idp:bob"),
			},
			NewIdentityGetterResponses: []interface{}{},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "identities") || actions[0].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("get", "users") || actions[1].(clienttesting.GetAction).GetName() != "bob" {
					t.Error(spew.Sdump(actions))
				}
			},
			ExpectedError: true,
		},
		"existing identity, user reference without identity backreference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingObjects: []runtime.Object{
				makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob"),
				makeUser("bobUserUID", "bob" /*, "idp:bob"*/),
			},
			NewIdentityGetterResponses: []interface{}{},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "identities") || actions[0].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("get", "users") || actions[1].(clienttesting.GetAction).GetName() != "bob" {
					t.Error(spew.Sdump(actions))
				}
			},
			ExpectedError: true,
		},
		"existing identity, user reference": {
			ProviderName:     "idp",
			ProviderUserName: "bob",

			ExistingObjects: []runtime.Object{
				makeIdentity("bobIdentityUID", "idp", "bob", "bobUserUID", "bob"),
				makeUser("bobUserUID", "bob", "idp:bob"),
			},
			NewIdentityGetterResponses: []interface{}{},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "identities") || actions[0].(clienttesting.GetAction).GetName() != "idp:bob" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("get", "users") || actions[1].(clienttesting.GetAction).GetName() != "bob" {
					t.Error(spew.Sdump(actions))
				}
			},
			ExpectedUserName: "bob",
		},
	}

	for k, tc := range testcases {
		t.Run(k, func(t *testing.T) {
			fakeClient := userv1fakeclient.NewSimpleClientset(tc.ExistingObjects...)

			newIdentityUserGetter := &testNewIdentityGetter{responses: tc.NewIdentityGetterResponses}
			provisionMapper := &provisioningIdentityMapper{
				identity:             fakeClient.UserV1().Identities(),
				user:                 fakeClient.UserV1().Users(),
				provisioningStrategy: newIdentityUserGetter,
			}

			identity := authapi.NewDefaultUserIdentityInfo(tc.ProviderName, tc.ProviderUserName)
			user, err := provisionMapper.UserFor(identity)
			if tc.ExpectedError != (err != nil) {
				t.Fatalf("Expected error=%v, got %v", tc.ExpectedError, err)
			}
			if !tc.ExpectedError && user.GetName() != tc.ExpectedUserName {
				t.Fatalf("Expected username %v, got %v", tc.ExpectedUserName, user.GetName())
			}

			if newIdentityUserGetter.called != len(tc.NewIdentityGetterResponses) {
				t.Errorf(" Expected %d calls to UserForNewIdentity, got %d", len(tc.NewIdentityGetterResponses), newIdentityUserGetter.called)
			}

			tc.ValidateActions(t, fakeClient.Actions())
		})

	}
}
