package identitymapper

import (
	"reflect"
	"testing"

	userapi "github.com/openshift/api/user/v1"
)

func TestInitializerUser(t *testing.T) {
	testcases := map[string]struct {
		Identity     *userapi.Identity
		User         *userapi.User
		ExpectedUser *userapi.User
	}{
		"empty": {
			Identity:     &userapi.Identity{},
			User:         &userapi.User{},
			ExpectedUser: &userapi.User{},
		},
		"empty extra": {
			Identity:     &userapi.Identity{Extra: map[string]string{}},
			User:         &userapi.User{},
			ExpectedUser: &userapi.User{},
		},
		"sets full name": {
			Identity: &userapi.Identity{
				Extra: map[string]string{"name": "Bob"},
			},
			User:         &userapi.User{},
			ExpectedUser: &userapi.User{FullName: "Bob"},
		},
		"respects existing full name": {
			Identity: &userapi.Identity{
				Extra: map[string]string{"name": "Bob"},
			},
			User:         &userapi.User{FullName: "Harold"},
			ExpectedUser: &userapi.User{FullName: "Harold"},
		},
	}

	for k, tc := range testcases {
		err := NewDefaultUserInitStrategy().InitializeUser(tc.Identity, tc.User)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		if !reflect.DeepEqual(tc.User, tc.ExpectedUser) {
			t.Errorf("%s: expected \n\t%#v\ngot\n\t%#v", k, tc.ExpectedUser, tc.User)
		}
	}
}
