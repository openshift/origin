package identitymapper

import (
	"testing"

	userapi "github.com/openshift/api/user/v1"
	"github.com/openshift/origin/pkg/user/registry/test"
)

func TestStrategyClaim(t *testing.T) {
	testcases := map[string]strategyTestCase{
		"no user": {
			MakeStrategy:      NewStrategyClaim,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			CreateResponse: makeUser("bobUserUID", "bob", "idp:bob"),

			ExpectedActions: []test.Action{
				{Name: "GetUser", Object: "bob"},
				{Name: "CreateUser", Object: makeUser("", "bob", "idp:bob")},
			},
			ExpectedUserName:   "bob",
			ExpectedInitialize: true,
		},
		"existing user, no identities": {
			MakeStrategy:      NewStrategyClaim,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers:  []*userapi.User{makeUser("bobUserUID", "bob")},
			UpdateResponse: makeUser("bobUserUID", "bob", "idp:bob"),

			ExpectedActions: []test.Action{
				{Name: "GetUser", Object: "bob"},
				{Name: "UpdateUser", Object: makeUser("bobUserUID", "bob", "idp:bob")},
			},
			ExpectedUserName:   "bob",
			ExpectedInitialize: true,
		},
		"existing user, conflicting identity": {
			MakeStrategy:      NewStrategyClaim,
			PreferredUsername: "bob",
			Identity:          makeIdentity("", "idp", "bob", "", ""),

			ExistingUsers:  []*userapi.User{makeUser("bobUserUID", "bob", "otheridp:user")},
			UpdateResponse: makeUser("bobUserUID", "bob", "otheridp:user", "idp:bob"),

			ExpectedActions: []test.Action{
				{Name: "GetUser", Object: "bob"},
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
