package scope

import (
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	defaultauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
)

func TestAuthorize(t *testing.T) {
	testCases := []struct {
		name                string
		user                user.Info
		attributes          defaultauthorizer.DefaultAuthorizationAttributes
		delegateAuthAllowed bool
		expectedCalled      bool
		expectedAllowed     bool
		expectedErr         string
		expectedMsg         string
	}{
		{
			name:        "no user",
			expectedErr: `user missing from context`,
		},
		{
			name:           "no extra",
			user:           &user.DefaultInfo{},
			expectedCalled: true,
		},
		{
			name:           "empty extra",
			user:           &user.DefaultInfo{Extra: map[string][]string{}},
			expectedCalled: true,
		},
		{
			name:           "empty scopes",
			user:           &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {}}},
			expectedCalled: true,
		},
		{
			name:        "bad scope",
			user:        &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"does-not-exist"}}},
			expectedMsg: `scopes [does-not-exist] prevent this action; User "" cannot "" "" with name "" in project "ns"`,
			expectedErr: `no scope evaluator found for "does-not-exist"`,
		},
		{
			name:        "bad scope 2",
			user:        &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:dne"}}},
			expectedMsg: `scopes [user:dne] prevent this action; User "" cannot "" "" with name "" in project "ns"`,
			expectedErr: `unrecognized scope: user:dne`,
		},
		{
			name:        "scope doesn't cover",
			user:        &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:info"}}},
			attributes:  defaultauthorizer.DefaultAuthorizationAttributes{Verb: "get", Resource: "users", ResourceName: "harold"},
			expectedMsg: `scopes [user:info] prevent this action; User "" cannot get users in project "ns"`,
		},
		{
			name:           "scope covers",
			user:           &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:info"}}},
			attributes:     defaultauthorizer.DefaultAuthorizationAttributes{Verb: "get", Resource: "users", ResourceName: "~"},
			expectedCalled: true,
		},
		{
			name:           "scope covers for discovery",
			user:           &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:info"}}},
			attributes:     defaultauthorizer.DefaultAuthorizationAttributes{Verb: "get", NonResourceURL: true, URL: "/api"},
			expectedCalled: true,
		},
		{
			name:           "user:full covers any resource",
			user:           &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:full"}}},
			attributes:     defaultauthorizer.DefaultAuthorizationAttributes{Verb: "update", Resource: "users", ResourceName: "harold"},
			expectedCalled: true,
		},
		{
			name:           "user:full covers any non-resource",
			user:           &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:full"}}},
			attributes:     defaultauthorizer.DefaultAuthorizationAttributes{Verb: "post", NonResourceURL: true, URL: "/foo/bar/baz"},
			expectedCalled: true,
		},
	}

	for _, tc := range testCases {
		delegate := &fakeAuthorizer{allowed: tc.delegateAuthAllowed}
		authorizer := NewAuthorizer(delegate, nil, defaultauthorizer.NewForbiddenMessageResolver(""))

		ctx := kapi.WithNamespace(kapi.NewContext(), "ns")
		if tc.user != nil {
			ctx = kapi.WithUser(ctx, tc.user)

		}

		actualAllowed, actualMsg, actualErr := authorizer.Authorize(ctx, tc.attributes)
		switch {
		case len(tc.expectedErr) == 0 && actualErr == nil:
		case len(tc.expectedErr) == 0 && actualErr != nil:
			t.Errorf("%s: unexpected error: %v", tc.name, actualErr)
		case len(tc.expectedErr) != 0 && actualErr == nil:
			t.Errorf("%s: missing error: %v", tc.name, tc.expectedErr)
		case len(tc.expectedErr) != 0 && actualErr != nil:
			if !strings.Contains(actualErr.Error(), tc.expectedErr) {
				t.Errorf("%s: expected %v, got %v", tc.name, tc.expectedErr, actualErr)
			}
		}
		if tc.expectedMsg != actualMsg {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.expectedMsg, actualMsg)
		}
		if tc.expectedAllowed != actualAllowed {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.expectedAllowed, actualAllowed)
		}
		if tc.expectedCalled != delegate.called {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.expectedCalled, delegate.called)
		}
	}
}

type fakeAuthorizer struct {
	allowed bool
	called  bool
}

func (a *fakeAuthorizer) Authorize(ctx kapi.Context, passedAttributes defaultauthorizer.Action) (bool, string, error) {
	a.called = true
	return a.allowed, "", nil
}

func (a *fakeAuthorizer) GetAllowedSubjects(ctx kapi.Context, attributes defaultauthorizer.Action) (sets.String, sets.String, error) {
	return nil, nil, nil
}
