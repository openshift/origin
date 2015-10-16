package identitymapper

import (
	"testing"

	"github.com/openshift/origin/pkg/user/api"
	"github.com/openshift/origin/pkg/user/registry/test"
)

func TestStrategyGenerate(t *testing.T) {
	testcases := map[string]strategyTestCase{
		"no user": {
			MakeStrategy:      NewStrategyGenerate,
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
			MakeStrategy:      NewStrategyGenerate,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers:  []*api.User{makeUser("bobUserUID", "bob")},
			CreateResponse: makeUser("bob2UserUID", "bob2", "idp:bob"),

			ExpectedActions: []test.Action{
				{"GetUser", "bob"},
				{"GetUser", "bob2"},
				{"CreateUser", makeUser("", "bob2", "idp:bob")},
			},
			ExpectedUserName:   "bob2",
			ExpectedInitialize: true,
		},
		"existing user, conflicting identity": {
			MakeStrategy:      NewStrategyGenerate,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers:  []*api.User{makeUser("bobUserUID", "bob", "otheridp:user")},
			CreateResponse: makeUser("bob2UserUID", "bob2", "idp:bob"),

			ExpectedActions: []test.Action{
				{"GetUser", "bob"},
				{"GetUser", "bob2"},
				{"CreateUser", makeUser("", "bob2", "idp:bob")},
			},
			ExpectedUserName:   "bob2",
			ExpectedInitialize: true,
		},
		"existing users": {
			MakeStrategy:      NewStrategyGenerate,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers: []*api.User{
				makeUser("bobUserUID", "bob", "otheridp:user"),
				makeUser("bob2UserUID", "bob2", "otheridp:user2"),
			},
			CreateResponse: makeUser("bob3UserUID", "bob3", "idp:bob"),

			ExpectedActions: []test.Action{
				{"GetUser", "bob"},
				{"GetUser", "bob2"},
				{"GetUser", "bob3"},
				{"CreateUser", makeUser("", "bob3", "idp:bob")},
			},
			ExpectedUserName:   "bob3",
			ExpectedInitialize: true,
		},
	}

	for testCaseName, testCase := range testcases {
		testCase.run(testCaseName, t)
	}
}
