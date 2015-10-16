package identitymapper

import (
	"testing"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/test"
)

func TestStrategyAdd(t *testing.T) {
	testcases := map[string]strategyTestCase{
		"no user": {
			MakeStrategy:      NewStrategyAdd,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			CreateResponse: makeUser("bobUserUID", "bob", "idp:bob"),

			ExpectedActions: []test.Action{
				{"GetUser", "bob"},
				{"CreateUser", makeUser("", "bob", "idp:bob")},
			},
			ExpectedUserName:   "bob",
			ExpectedInitialize: true,
		},
		"existing user, no identities": {
			MakeStrategy:      NewStrategyAdd,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers:  []*api.User{makeUser("bobUserUID", "bob")},
			UpdateResponse: makeUser("bobUserUID", "bob", "idp:bob"),

			ExpectedActions: []test.Action{
				{"GetUser", "bob"},
				{"UpdateUser", makeUser("bobUserUID", "bob", "idp:bob")},
			},
			ExpectedUserName:   "bob",
			ExpectedInitialize: true,
		},
		"existing user, conflicting identity": {
			MakeStrategy:      NewStrategyAdd,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers:  []*api.User{makeUser("bobUserUID", "bob", "otheridp:user")},
			UpdateResponse: makeUser("bobUserUID", "bob", "otheridp:user", "idp:bob"),

			ExpectedActions: []test.Action{
				{"GetUser", "bob"},
				{"UpdateUser", makeUser("bobUserUID", "bob", "otheridp:user", "idp:bob")},
			},
			ExpectedUserName:   "bob",
			ExpectedInitialize: false,
		},
	}

	for testCaseName, testCase := range testcases {
		testCase.run(testCaseName, t)
	}
}
