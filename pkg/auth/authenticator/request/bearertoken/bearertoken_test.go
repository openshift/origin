package bearertoken

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/auth/user"
)

func TestBearerToken(t *testing.T) {
	tests := map[string]struct {
		AuthorizationHeaders []string
		TokenAuth            authenticator.Token
		RemoveHeader         bool

		ExpectedUserName             string
		ExpectedOK                   bool
		ExpectedErr                  bool
		ExpectedAuthorizationHeaders []string
	}{
		"no header": {
			AuthorizationHeaders:         nil,
			RemoveHeader:                 true,
			ExpectedUserName:             "",
			ExpectedOK:                   false,
			ExpectedErr:                  false,
			ExpectedAuthorizationHeaders: nil,
		},
		"empty header": {
			AuthorizationHeaders:         []string{""},
			RemoveHeader:                 true,
			ExpectedUserName:             "",
			ExpectedOK:                   false,
			ExpectedErr:                  false,
			ExpectedAuthorizationHeaders: []string{""},
		},
		"non-bearer header": {
			AuthorizationHeaders:         []string{"Basic 123"},
			RemoveHeader:                 true,
			ExpectedUserName:             "",
			ExpectedOK:                   false,
			ExpectedErr:                  false,
			ExpectedAuthorizationHeaders: []string{"Basic 123"},
		},
		"empty bearer token": {
			AuthorizationHeaders:         []string{"Bearer "},
			RemoveHeader:                 true,
			ExpectedUserName:             "",
			ExpectedOK:                   false,
			ExpectedErr:                  false,
			ExpectedAuthorizationHeaders: []string{"Bearer "},
		},
		"valid bearer token removing header": {
			AuthorizationHeaders:         []string{"Bearer 123"},
			TokenAuth:                    authenticator.TokenFunc(func(t string) (user.Info, bool, error) { return &user.DefaultInfo{Name: "myuser"}, true, nil }),
			RemoveHeader:                 true,
			ExpectedUserName:             "myuser",
			ExpectedOK:                   true,
			ExpectedErr:                  false,
			ExpectedAuthorizationHeaders: nil,
		},
		"valid bearer token leaving header": {
			AuthorizationHeaders:         []string{"Bearer 123"},
			TokenAuth:                    authenticator.TokenFunc(func(t string) (user.Info, bool, error) { return &user.DefaultInfo{Name: "myuser"}, true, nil }),
			RemoveHeader:                 false,
			ExpectedUserName:             "myuser",
			ExpectedOK:                   true,
			ExpectedErr:                  false,
			ExpectedAuthorizationHeaders: []string{"Bearer 123"},
		},
		"invalid bearer token": {
			AuthorizationHeaders:         []string{"Bearer 123"},
			TokenAuth:                    authenticator.TokenFunc(func(t string) (user.Info, bool, error) { return nil, false, nil }),
			RemoveHeader:                 true,
			ExpectedUserName:             "",
			ExpectedOK:                   false,
			ExpectedErr:                  false,
			ExpectedAuthorizationHeaders: []string{"Bearer 123"},
		},
		"error bearer token": {
			AuthorizationHeaders:         []string{"Bearer 123"},
			TokenAuth:                    authenticator.TokenFunc(func(t string) (user.Info, bool, error) { return nil, false, errors.New("error") }),
			RemoveHeader:                 true,
			ExpectedUserName:             "",
			ExpectedOK:                   false,
			ExpectedErr:                  true,
			ExpectedAuthorizationHeaders: []string{"Bearer 123"},
		},
	}

	for k, tc := range tests {
		req, _ := http.NewRequest("GET", "/", nil)
		for _, h := range tc.AuthorizationHeaders {
			req.Header.Add("Authorization", h)
		}

		bearerAuth := New(tc.TokenAuth, tc.RemoveHeader)
		u, ok, err := bearerAuth.AuthenticateRequest(req)
		if tc.ExpectedErr != (err != nil) {
			t.Errorf("%s: Expected err=%v, got %v", k, tc.ExpectedErr, err)
			continue
		}
		if ok != tc.ExpectedOK {
			t.Errorf("%s: Expected ok=%v, got %v", k, tc.ExpectedOK, ok)
			continue
		}
		if ok && u.GetName() != tc.ExpectedUserName {
			t.Errorf("%s: Expected username=%v, got %v", k, tc.ExpectedUserName, u.GetName())
			continue
		}
		if !reflect.DeepEqual(req.Header["Authorization"], tc.ExpectedAuthorizationHeaders) {
			t.Errorf("%s: Expected headers=%#v, got %#v", k, tc.ExpectedAuthorizationHeaders, req.Header["Authorization"])
			continue
		}
	}
}
