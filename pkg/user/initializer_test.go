package user

import (
	"reflect"
	"testing"

	"github.com/openshift/origin/pkg/user/api"
)

func TestInitializerUser(t *testing.T) {
	testcases := map[string]struct {
		Identity     *api.Identity
		User         *api.User
		ExpectedUser *api.User
	}{
		"empty": {
			Identity:     &api.Identity{},
			User:         &api.User{},
			ExpectedUser: &api.User{},
		},
		"empty extra": {
			Identity:     &api.Identity{Extra: map[string]string{}},
			User:         &api.User{},
			ExpectedUser: &api.User{},
		},
		"sets full name": {
			Identity: &api.Identity{
				Extra: map[string]string{"name": "Bob"},
			},
			User:         &api.User{},
			ExpectedUser: &api.User{FullName: "Bob"},
		},
		"respects existing full name": {
			Identity: &api.Identity{
				Extra: map[string]string{"name": "Bob"},
			},
			User:         &api.User{FullName: "Harold"},
			ExpectedUser: &api.User{FullName: "Harold"},
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
