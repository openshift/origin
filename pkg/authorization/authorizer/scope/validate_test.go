package scope

import (
	"strings"
	"testing"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

func TestValidateScopeRestrictions(t *testing.T) {
	testCases := []struct {
		name   string
		scopes []string
		client *oauthapi.OAuthClient

		expectedErrors []string
	}{
		{
			name:   "unrestricted allows any",
			scopes: []string{"one"},
			client: &oauthapi.OAuthClient{AllowAnyScope: true},
		},
		{
			name:   "unrestricted allows empty",
			scopes: []string{""},
			client: &oauthapi.OAuthClient{AllowAnyScope: true},
		},
		{
			name:   "unrestricted allows none",
			scopes: []string{},
			client: &oauthapi.OAuthClient{AllowAnyScope: true},
		},
		{
			name:           "no restrictions denies any",
			scopes:         []string{"one"},
			client:         &oauthapi.OAuthClient{},
			expectedErrors: []string{`one did not match any scope restriction`},
		},
		{
			name:           "no restrictions denies empty",
			scopes:         []string{""},
			client:         &oauthapi.OAuthClient{},
			expectedErrors: []string{`did not match any scope restriction`},
		},
		{
			name:           "no restrictions denies none",
			scopes:         []string{},
			client:         &oauthapi.OAuthClient{},
			expectedErrors: []string{`may not request unscoped tokens`},
		},
		{
			name:   "simple literal",
			scopes: []string{"one"},
			client: &oauthapi.OAuthClient{
				ScopeRestrictions: []oauthapi.ScopeRestriction{{ExactValues: []string{"two", "one"}}},
			},
		},
		{
			name:   "simple must match",
			scopes: []string{"missing"},
			client: &oauthapi.OAuthClient{
				ScopeRestrictions: []oauthapi.ScopeRestriction{{ExactValues: []string{"two", "one"}}},
			},
			expectedErrors: []string{`missing not found in [two one]`},
		},
		{
			name:   "cluster role name must match",
			scopes: []string{ClusterRoleIndicator + "three:alfa"},
			client: &oauthapi.OAuthClient{
				ScopeRestrictions: []oauthapi.ScopeRestriction{{ClusterRole: &oauthapi.ClusterRoleScopeRestriction{
					RoleNames:       []string{"one", "two"},
					Namespaces:      []string{"alfa", "bravo"},
					AllowEscalation: false,
				}}},
			},
			expectedErrors: []string{`role:three:alfa does not use an approved name`},
		},
		{
			name:   "cluster role namespace must match",
			scopes: []string{ClusterRoleIndicator + "two:charlie"},
			client: &oauthapi.OAuthClient{
				ScopeRestrictions: []oauthapi.ScopeRestriction{{ClusterRole: &oauthapi.ClusterRoleScopeRestriction{
					RoleNames:       []string{"one", "two"},
					Namespaces:      []string{"alfa", "bravo"},
					AllowEscalation: false,
				}}},
			},
			expectedErrors: []string{`role:two:charlie does not use an approved namespace`},
		},
		{
			name:   "cluster role escalation must match",
			scopes: []string{ClusterRoleIndicator + "two:bravo:!"},
			client: &oauthapi.OAuthClient{
				ScopeRestrictions: []oauthapi.ScopeRestriction{{ClusterRole: &oauthapi.ClusterRoleScopeRestriction{
					RoleNames:       []string{"one", "two"},
					Namespaces:      []string{"alfa", "bravo"},
					AllowEscalation: false,
				}}},
			},
			expectedErrors: []string{`role:two:bravo:! is not allowed to escalate`},
		},
		{
			name:   "cluster role matches",
			scopes: []string{ClusterRoleIndicator + "two:bravo:!"},
			client: &oauthapi.OAuthClient{
				ScopeRestrictions: []oauthapi.ScopeRestriction{{ClusterRole: &oauthapi.ClusterRoleScopeRestriction{
					RoleNames:       []string{"one", "two"},
					Namespaces:      []string{"alfa", "bravo"},
					AllowEscalation: true,
				}}},
			},
		},
		{
			name:   "cluster role matches 2",
			scopes: []string{ClusterRoleIndicator + "two:bravo"},
			client: &oauthapi.OAuthClient{
				ScopeRestrictions: []oauthapi.ScopeRestriction{{ClusterRole: &oauthapi.ClusterRoleScopeRestriction{
					RoleNames:       []string{"one", "two"},
					Namespaces:      []string{"alfa", "bravo"},
					AllowEscalation: false,
				}}},
			},
		},
		{
			name:   "cluster role star matches",
			scopes: []string{ClusterRoleIndicator + "two:bravo"},
			client: &oauthapi.OAuthClient{
				ScopeRestrictions: []oauthapi.ScopeRestriction{{ClusterRole: &oauthapi.ClusterRoleScopeRestriction{
					RoleNames:       []string{"one", "two", "*"},
					Namespaces:      []string{"alfa", "bravo", "*"},
					AllowEscalation: true,
				}}},
			},
		},
	}

	for _, tc := range testCases {
		err := ValidateScopeRestrictions(tc.client, tc.scopes...)
		if err != nil && len(tc.expectedErrors) == 0 {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if err == nil && len(tc.expectedErrors) > 0 {
			t.Errorf("%s: missing error: %v", tc.name, tc.expectedErrors)
			continue
		}
		if err == nil && len(tc.expectedErrors) == 0 {
			continue
		}

		for _, expectedErr := range tc.expectedErrors {
			if !strings.Contains(err.Error(), expectedErr) {
				t.Errorf("%s: error %v missing %v", tc.name, err, expectedErr)
			}
		}
	}

}
