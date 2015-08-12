package headerrequest

import (
	"net/http"
	"testing"

	"github.com/openshift/origin/pkg/auth/api"
	"k8s.io/kubernetes/pkg/auth/user"
)

type TestUserIdentityMapper struct{}

func (m *TestUserIdentityMapper) UserFor(identityInfo api.UserIdentityInfo) (user.Info, error) {
	return &user.DefaultInfo{Name: identityInfo.GetProviderUserName()}, nil
}

func TestRequestHeader(t *testing.T) {
	testcases := map[string]struct {
		ConfiguredHeaders []string
		RequestHeaders    http.Header
		ExpectedUsername  string
	}{
		"empty": {
			ExpectedUsername: "",
		},
		"no match": {
			ConfiguredHeaders: []string{"X-Remote-User"},
			ExpectedUsername:  "",
		},
		"match": {
			ConfiguredHeaders: []string{"X-Remote-User"},
			RequestHeaders:    http.Header{"X-Remote-User": {"Bob"}},
			ExpectedUsername:  "Bob",
		},
		"exact match": {
			ConfiguredHeaders: []string{"X-Remote-User"},
			RequestHeaders: http.Header{
				"Prefixed-X-Remote-User-With-Suffix": {"Bob"},
				"X-Remote-User-With-Suffix":          {"Bob"},
			},
			ExpectedUsername: "",
		},
		"first match": {
			ConfiguredHeaders: []string{
				"X-Remote-User",
				"A-Second-X-Remote-User",
				"Another-X-Remote-User",
			},
			RequestHeaders: http.Header{
				"X-Remote-User":          {"", "First header, second value"},
				"A-Second-X-Remote-User": {"Second header, first value", "Second header, second value"},
				"Another-X-Remote-User":  {"Third header, first value"}},
			ExpectedUsername: "Second header, first value",
		},
		"case-insensitive": {
			ConfiguredHeaders: []string{"x-REMOTE-user"},             // configured headers can be case-insensitive
			RequestHeaders:    http.Header{"X-Remote-User": {"Bob"}}, // the parsed headers are normalized by the http package
			ExpectedUsername:  "Bob",
		},
	}

	for k, testcase := range testcases {
		mapper := &TestUserIdentityMapper{}
		auth := NewAuthenticator("testprovider", &Config{testcase.ConfiguredHeaders}, mapper)
		req := &http.Request{Header: testcase.RequestHeaders}

		user, ok, err := auth.AuthenticateRequest(req)
		if testcase.ExpectedUsername == "" {
			if ok {
				t.Errorf("%s: Didn't expect user, authentication succeeded", k)
				continue
			}
		}
		if testcase.ExpectedUsername != "" {
			if err != nil {
				t.Errorf("%s: Expected user, got error: %v", k, err)
				continue
			}
			if !ok {
				t.Errorf("%s: Expected user, auth failed", k)
				continue
			}
			if testcase.ExpectedUsername != user.GetName() {
				t.Errorf("%s: Expected username %s, got %s", k, testcase.ExpectedUsername, user.GetName())
				continue
			}
		}
	}
}
