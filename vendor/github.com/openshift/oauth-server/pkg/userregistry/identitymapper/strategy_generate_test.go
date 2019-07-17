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

func TestStrategyGenerate(t *testing.T) {
	testcases := map[string]strategyTestCase{
		"no user": {
			MakeStrategy:      NewStrategyGenerate,
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
			MakeStrategy:      NewStrategyGenerate,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers: []runtime.Object{makeUser("bobUserUID", "bob")},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 3 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "users") || actions[0].(clienttesting.GetAction).GetName() != "bob" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("get", "users") || actions[1].(clienttesting.GetAction).GetName() != "bob2" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[2].Matches("create", "users") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[2].(clienttesting.CreateAction).GetObject().(*userapi.User)
				if expected := makeUser("", "bob2", "idp:bob"); !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
			ExpectedUserName:   "bob2",
			ExpectedInitialize: true,
		},
		"existing user, conflicting identity": {
			MakeStrategy:      NewStrategyGenerate,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers: []runtime.Object{makeUser("bobUserUID", "bob", "otheridp:user")},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 3 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "users") || actions[0].(clienttesting.GetAction).GetName() != "bob" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("get", "users") || actions[1].(clienttesting.GetAction).GetName() != "bob2" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[2].Matches("create", "users") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[2].(clienttesting.CreateAction).GetObject().(*userapi.User)
				if expected := makeUser("", "bob2", "idp:bob"); !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},
			ExpectedUserName:   "bob2",
			ExpectedInitialize: true,
		},
		"existing users": {
			MakeStrategy:      NewStrategyGenerate,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers: []runtime.Object{
				makeUser("bobUserUID", "bob", "otheridp:user"),
				makeUser("bob2UserUID", "bob2", "otheridp:user2"),
			},

			ValidateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 4 {
					t.Fatal(spew.Sdump(actions))
				}
				if !actions[0].Matches("get", "users") || actions[0].(clienttesting.GetAction).GetName() != "bob" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[1].Matches("get", "users") || actions[1].(clienttesting.GetAction).GetName() != "bob2" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[2].Matches("get", "users") || actions[2].(clienttesting.GetAction).GetName() != "bob3" {
					t.Error(spew.Sdump(actions))
				}
				if !actions[3].Matches("create", "users") {
					t.Error(spew.Sdump(actions))
				}
				actual := actions[3].(clienttesting.CreateAction).GetObject().(*userapi.User)
				if expected := makeUser("", "bob3", "idp:bob"); !equality.Semantic.DeepEqual(expected, actual) {
					t.Error(diff.ObjectDiff(expected, actual))
				}
			},

			ExpectedUserName:   "bob3",
			ExpectedInitialize: true,
		},
	}

	for testCaseName, testCase := range testcases {
		testCase.run(testCaseName, t)
	}
}
