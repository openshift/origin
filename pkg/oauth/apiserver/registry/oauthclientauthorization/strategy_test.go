package oauthclientauthorization

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1"

	oauth "github.com/openshift/api/oauth/v1"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

// TestValidateScopeUpdate asserts that a live client lookup only occurs when new scopes are added during an update
func TestValidateScopeUpdate(t *testing.T) {
	for _, test := range []struct {
		name           string
		expectedCalled bool
		obj            []string
		old            []string
	}{
		{
			name:           "both empty",
			expectedCalled: false,
			obj:            []string{},
			old:            []string{},
		},
		{
			name:           "both equal",
			expectedCalled: false,
			obj:            []string{scope.UserAccessCheck},
			old:            []string{scope.UserAccessCheck},
		},
		{
			name:           "new scopes from empty",
			expectedCalled: true,
			obj:            []string{scope.UserFull},
			old:            []string{},
		},
		{
			name:           "new scopes from non-empty",
			expectedCalled: true,
			obj:            []string{scope.UserFull},
			old:            []string{scope.UserInfo},
		},
		{
			name:           "deleted scopes",
			expectedCalled: false,
			obj:            []string{scope.UserFull},
			old:            []string{scope.UserFull, scope.UserInfo},
		},
		{
			name:           "deleted and added scopes",
			expectedCalled: true,
			obj:            []string{scope.UserFull, scope.UserAccessCheck},
			old:            []string{scope.UserFull, scope.UserInfo},
		},
		{
			name:           "deleted scopes to empty",
			expectedCalled: true,
			obj:            []string{},
			old:            []string{scope.UserFull},
		},
	} {
		clientGetter := &wasCalledClientGetter{}
		s := strategy{clientGetter: clientGetter}
		if errs := s.ValidateUpdate(nil, validClientWithScopes(test.obj), validClientWithScopes(test.old)); len(errs) > 0 {
			t.Errorf("%s: unexpected update error: %s", test.name, errs)
			continue
		}
		if test.expectedCalled != clientGetter.called {
			t.Errorf("%s: expected call behavior %v does not match %v", test.name, test.expectedCalled, clientGetter.called)
		}
	}
}

type wasCalledClientGetter struct {
	called bool
}

func (g *wasCalledClientGetter) Get(_ string, _ v1.GetOptions) (*oauth.OAuthClient, error) {
	g.called = true
	return &oauth.OAuthClient{}, nil
}

func validClientWithScopes(scopes []string) *oauthapi.OAuthClientAuthorization {
	return &oauthapi.OAuthClientAuthorization{
		ObjectMeta: v1.ObjectMeta{Name: "un:cn", ResourceVersion: "0"},
		ClientName: "cn",
		UserName:   "un",
		UserUID:    "uid",
		Scopes:     scopes,
	}
}
