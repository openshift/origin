package allowanypassword

import (
	"context"
	"testing"

	"github.com/openshift/oauth-server/pkg/api"
	"k8s.io/apiserver/pkg/authentication/user"
)

type testUserIdentityMapper struct{}

func (m *testUserIdentityMapper) UserFor(identityInfo api.UserIdentityInfo) (user.Info, error) {
	return &user.DefaultInfo{Name: identityInfo.GetProviderUserName()}, nil
}

func TestAnyAuthPassword(t *testing.T) {
	a := New("foo", &testUserIdentityMapper{})

	testcases := map[string]struct {
		Username         string
		Password         string
		ExpectedUsername string
		ExpectedOK       bool
		ExpectedErr      bool
	}{
		"empty user invalid": {
			Username:   "",
			Password:   "foo",
			ExpectedOK: false,
		},
		"empty password invalid": {
			Username:   "foo",
			Password:   "",
			ExpectedOK: false,
		},
		"valid username and password": {
			Username:         "foo",
			Password:         "bar",
			ExpectedOK:       true,
			ExpectedUsername: "foo",
		},
		"case-sensitive username": {
			Username:         "FOO",
			Password:         "bar",
			ExpectedOK:       true,
			ExpectedUsername: "FOO",
		},
		"whitespace-normalizing username": {
			Username:         "  FOO   BAR  ",
			Password:         "bar",
			ExpectedOK:       true,
			ExpectedUsername: "FOO   BAR",
		},
		"whitespace-only user invalid": {
			Username:   "  ",
			Password:   "bar",
			ExpectedOK: false,
		},
	}

	for k, tc := range testcases {
		response, ok, err := a.AuthenticatePassword(context.TODO(), tc.Username, tc.Password)
		if tc.ExpectedErr != (err != nil) {
			t.Errorf("%s: expected error=%v, got %v", k, tc.ExpectedErr, err)
			continue
		}
		if tc.ExpectedOK != ok {
			t.Errorf("%s: expected ok=%v, got %v", k, tc.ExpectedOK, ok)
			continue
		}
		username := ""
		if ok {
			username = response.User.GetName()
		}
		if tc.ExpectedUsername != username {
			t.Errorf("%s: expected username=%v, got %v", k, tc.ExpectedUsername, username)
			continue
		}
	}
}
