package scope

import (
	"strings"
	"testing"

	kauthorizer "k8s.io/kubernetes/pkg/auth/authorizer"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	defaultauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
)

func TestAuthorize(t *testing.T) {
	testCases := []struct {
		name                string
		attributes          kauthorizer.AttributesRecord
		delegateAuthAllowed bool
		expectedCalled      bool
		expectedAllowed     bool
		expectedErr         string
		expectedMsg         string
	}{
		{
			name: "no user",
			attributes: kauthorizer.AttributesRecord{
				ResourceRequest: true,
				Namespace:       "ns",
			},
			expectedErr: `user missing from context`,
		},
		{
			name: "no extra",
			attributes: kauthorizer.AttributesRecord{
				User:            &user.DefaultInfo{},
				ResourceRequest: true,
				Namespace:       "ns",
			},
			expectedCalled: true,
		},
		{
			name: "empty extra",
			attributes: kauthorizer.AttributesRecord{
				User:            &user.DefaultInfo{Extra: map[string][]string{}},
				ResourceRequest: true,
				Namespace:       "ns",
			},
			expectedCalled: true,
		},
		{
			name: "empty scopes",
			attributes: kauthorizer.AttributesRecord{
				User:            &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {}}},
				ResourceRequest: true,
				Namespace:       "ns",
			},
			expectedCalled: true,
		},
		{
			name: "bad scope",
			attributes: kauthorizer.AttributesRecord{
				User:            &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"does-not-exist"}}},
				ResourceRequest: true,
				Namespace:       "ns",
			},
			expectedMsg: `scopes [does-not-exist] prevent this action; User "" cannot "" "" with name "" in project "ns"`,
			expectedErr: `no scope evaluator found for "does-not-exist"`,
		},
		{
			name: "bad scope 2",
			attributes: kauthorizer.AttributesRecord{
				User:            &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:dne"}}},
				ResourceRequest: true,
				Namespace:       "ns",
			},
			expectedMsg: `scopes [user:dne] prevent this action; User "" cannot "" "" with name "" in project "ns"`,
			expectedErr: `unrecognized scope: user:dne`,
		},
		{
			name: "scope doesn't cover",
			attributes: kauthorizer.AttributesRecord{
				User:            &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:info"}}},
				ResourceRequest: true,
				Namespace:       "ns",
				Verb:            "get", Resource: "users", Name: "harold"},
			expectedMsg: `scopes [user:info] prevent this action; User "" cannot get users in project "ns"`,
		},
		{
			name: "scope covers",
			attributes: kauthorizer.AttributesRecord{
				User:            &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:info"}}},
				ResourceRequest: true,
				Namespace:       "ns",
				Verb:            "get", Resource: "users", Name: "~"},
			expectedCalled: true,
		},
		{
			name: "scope covers for discovery",
			attributes: kauthorizer.AttributesRecord{
				User:            &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:info"}}},
				ResourceRequest: false,
				Namespace:       "ns",
				Verb:            "get", Path: "/api"},
			expectedCalled: true,
		},
		{
			name: "user:full covers any resource",
			attributes: kauthorizer.AttributesRecord{
				User:            &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:full"}}},
				ResourceRequest: true,
				Namespace:       "ns",
				Verb:            "update", Resource: "users", Name: "harold"},
			expectedCalled: true,
		},
		{
			name: "user:full covers any non-resource",
			attributes: kauthorizer.AttributesRecord{
				User:            &user.DefaultInfo{Extra: map[string][]string{authorizationapi.ScopesKey: {"user:full"}}},
				ResourceRequest: false,
				Namespace:       "ns",
				Verb:            "post", Path: "/foo/bar/baz"},
			expectedCalled: true,
		},
	}

	for _, tc := range testCases {
		delegate := &fakeAuthorizer{allowed: tc.delegateAuthAllowed}
		authorizer := NewAuthorizer(delegate, nil, defaultauthorizer.NewForbiddenMessageResolver(""))

		actualAllowed, actualMsg, actualErr := authorizer.Authorize(tc.attributes)
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

func (a *fakeAuthorizer) Authorize(passedAttributes kauthorizer.Attributes) (bool, string, error) {
	a.called = true
	return a.allowed, "", nil
}

func (a *fakeAuthorizer) GetAllowedSubjects(attributes kauthorizer.Attributes) (sets.String, sets.String, error) {
	return nil, nil, nil
}
