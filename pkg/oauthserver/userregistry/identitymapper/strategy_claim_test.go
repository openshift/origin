package identitymapper

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	clienttesting "k8s.io/client-go/testing"

	"github.com/davecgh/go-spew/spew"
	userapi "github.com/openshift/api/user/v1"
)

func TestStrategyClaim(t *testing.T) {
	testcases := map[string]strategyTestCase{
		"no user": {
			MakeStrategy:      NewStrategyClaim,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "users") || actions[0].(clienttesting.GetAction).GetName() != "bob" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("create", "users") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[1].(clienttesting.CreateAction).GetObject().(*userapi.User)
				if expected := makeUser("", "bob", "idp:bob"); !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
			ExpectedUserName:   "bob",
			ExpectedInitialize: true,
		},
		"existing user, no identities": {
			MakeStrategy:      NewStrategyClaim,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers: []runtime.Object{makeUser("bobUserUID", "bob")},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 2 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "users") || actions[0].(clienttesting.GetAction).GetName() != "bob" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("update", "users") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[1].(clienttesting.CreateAction).GetObject().(*userapi.User)
				if expected := makeUser("bobUserUID", "bob", "idp:bob"); !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
			ExpectedUserName:   "bob",
			ExpectedInitialize: true,
		},
		"existing user, conflicting identity": {
			MakeStrategy:      NewStrategyClaim,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers: []runtime.Object{makeUser("bobUserUID", "bob", "otheridp:user")},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 1 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "users") || actions[0].(clienttesting.GetAction).GetName() != "bob" {
					t.Error(spew.Sdump(actions))
				}
			},
			ExpectedError:      true,
			ExpectedUserName:   "",
			ExpectedInitialize: false,
		},
	}

	for testCaseName, testCase := range testcases {
		testCase.run(testCaseName, t)
	}
}
