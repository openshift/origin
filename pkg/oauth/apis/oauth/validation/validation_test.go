package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

func TestValidateRedirectURI(t *testing.T) {
	allowed := []string{
		// Non-absolute
		"server",

		// No protocol
		"//server",

		// Case insensitive
		"HTTP://server",
		"HTTPS://server",

		// Normal paths
		"https://server",
		"https://server",

		// With ports
		"https://server:",
		"https://server:12345",

		// With or without paths, with or without trailing slashes
		"https://server:12345/",
		"https://server:12345/path-segment",
		"https://server:12345/path-segment/",

		// Things that are close to disallowed path segments
		"https://server:12345/...",
		"https://server:12345/.../",
		"https://server:12345/path-segment/...",
		"https://server:12345/path-segment/path.",
		"https://server:12345/path-segment/path./",

		// Double slashes
		"https://server:12345/path-segment//path",

		// Queries
		"http://server/path?",
		"http://server/path?query",
		"http://server/path?query=value",

		// Empty fragments
		"http://server/path?query=value#",
	}
	for i, u := range allowed {
		ok, msg := ValidateRedirectURI(u)
		if !ok {
			t.Errorf("%d expected %q to be allowed, but got error message %q", i, u, msg)
		}
	}

	disallowed := []string{
		// Empty disallowed
		"",

		// invalid URL
		"://server:12345/",

		// . or .. segments
		"http://server/.",
		"http://server/./",
		"http://server/..",
		"http://server/../",
		"http://server/path/..",
		"http://server/path/../",
		"http://server/path/../path",

		// Fragments
		"http://server/path?query#test",
	}
	for i, u := range disallowed {
		ok, _ := ValidateRedirectURI(u)
		if ok {
			t.Errorf("%d expected %q to be disallowed", i, u)
		}
	}
}

func TestValidateClientAuthorization(t *testing.T) {
	errs := ValidateClientAuthorization(&oauthapi.OAuthClientAuthorization{
		ObjectMeta: metav1.ObjectMeta{Name: "myusername:myclientname"},
		ClientName: "myclientname",
		UserName:   "myusername",
		UserUID:    "myuseruid",
		Scopes:     []string{"user:info"},
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A oauthapi.OAuthClientAuthorization
		T field.ErrorType
		F string
	}{
		"zero-length name": {
			A: oauthapi.OAuthClientAuthorization{
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{"user:info"},
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid name": {
			A: oauthapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "anotheruser:anotherclient"},
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{"user:info"},
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.name",
		},
		"disallowed namespace": {
			A: oauthapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "myusername:myclientname", Namespace: "foo"},
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{"user:info"},
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
		},
		"no scope handler": {
			A: oauthapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "myusername:myclientname"},
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{"invalid"},
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
		},
		"bad scope": {
			A: oauthapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "myusername:myclientname"},
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{"user:dne"},
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
		},
	}
	for k, v := range errorCases {
		errs := ValidateClientAuthorization(&v.A)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s  GOT: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s  GOT: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateClient(t *testing.T) {
	errs := ValidateClient(&oauthapi.OAuthClient{
		ObjectMeta:  metav1.ObjectMeta{Name: "client-name"},
		GrantMethod: "prompt",
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	var badTimeout int32 = MinimumInactivityTimeoutSeconds - 1
	var negTimeout int32 = -1

	errorCases := map[string]struct {
		Client oauthapi.OAuthClient
		T      field.ErrorType
		F      string
	}{
		"zero-length name": {
			Client: oauthapi.OAuthClient{
				GrantMethod: "prompt",
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"no grant method": {
			Client: oauthapi.OAuthClient{
				ObjectMeta: metav1.ObjectMeta{Name: "name"},
			},
			T: field.ErrorTypeRequired,
			F: "grantMethod",
		},
		"disallowed namespace": {
			Client: oauthapi.OAuthClient{
				ObjectMeta:  metav1.ObjectMeta{Name: "name", Namespace: "foo"},
				GrantMethod: "prompt",
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
		},
		"literal must have value": {
			Client: oauthapi.OAuthClient{
				ObjectMeta:        metav1.ObjectMeta{Name: "client-name"},
				GrantMethod:       "auto",
				ScopeRestrictions: []oauthapi.ScopeRestriction{{ExactValues: []string{""}}},
			},
			T: field.ErrorTypeInvalid,
			F: "scopeRestrictions[0].literals[0]",
		},
		"must have role names": {
			Client: oauthapi.OAuthClient{
				ObjectMeta:  metav1.ObjectMeta{Name: "client-name"},
				GrantMethod: "auto",
				ScopeRestrictions: []oauthapi.ScopeRestriction{
					{
						ClusterRole: &oauthapi.ClusterRoleScopeRestriction{Namespaces: []string{"b"}},
					},
				},
			},
			T: field.ErrorTypeRequired,
			F: "scopeRestrictions[0].clusterRole.roleNames",
		},
		"must have namespaces": {
			Client: oauthapi.OAuthClient{
				ObjectMeta:  metav1.ObjectMeta{Name: "client-name"},
				GrantMethod: "prompt",
				ScopeRestrictions: []oauthapi.ScopeRestriction{
					{
						ClusterRole: &oauthapi.ClusterRoleScopeRestriction{RoleNames: []string{"a"}},
					},
				},
			},
			T: field.ErrorTypeRequired,
			F: "scopeRestrictions[0].clusterRole.namespaces",
		},
		"minimum timeout value": {
			Client: oauthapi.OAuthClient{
				ObjectMeta:                          metav1.ObjectMeta{Name: "client-name"},
				GrantMethod:                         "auto",
				AccessTokenInactivityTimeoutSeconds: &badTimeout,
			},
			T: field.ErrorTypeInvalid,
			F: "accessTokenInactivityTimeoutSeconds",
		},
		"negative timeout value": {
			Client: oauthapi.OAuthClient{
				ObjectMeta:                          metav1.ObjectMeta{Name: "client-name"},
				GrantMethod:                         "auto",
				AccessTokenInactivityTimeoutSeconds: &negTimeout,
			},
			T: field.ErrorTypeInvalid,
			F: "accessTokenInactivityTimeoutSeconds",
		},
	}
	for k, v := range errorCases {
		errs := ValidateClient(&v.Client)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.Client)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateAccessTokens(t *testing.T) {
	errs := ValidateAccessToken(&oauthapi.OAuthAccessToken{
		ObjectMeta:  metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength"},
		ClientName:  "myclient",
		UserName:    "myusername",
		UserUID:     "myuseruid",
		Scopes:      []string{"user:full"},
		RedirectURI: "https://authn.mycluster.com",
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		Token oauthapi.OAuthAccessToken
		T     field.ErrorType
		F     string
	}{
		"zero-length name": {
			Token: oauthapi.OAuthAccessToken{
				ClientName:  "myclient",
				UserName:    "myusername",
				UserUID:     "myuseruid",
				Scopes:      []string{"user:full"},
				RedirectURI: "https://authn.mycluster.com",
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"empty redirect uri": {
			Token: oauthapi.OAuthAccessToken{
				ObjectMeta: metav1.ObjectMeta{Name: "myperfectTokenWithFullScopeAndStuff"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{"user:full"},
			},
			T: field.ErrorTypeInvalid,
			F: "redirectURI",
		},
		"empty scopes": {
			Token: oauthapi.OAuthAccessToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "myperfectTokenWithFullScopeAndStuff"},
				ClientName:  "myclient",
				UserName:    "myusername",
				UserUID:     "myuseruid",
				RedirectURI: "https://authn.mycluster.com",
			},
			T: field.ErrorTypeRequired,
			F: "scopes",
		},
		"disallowed namespace": {
			Token: oauthapi.OAuthAccessToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength", Namespace: "foo"},
				ClientName:  "myclient",
				UserName:    "myusername",
				UserUID:     "myuseruid",
				Scopes:      []string{"user:full"},
				RedirectURI: "https://authn.mycluster.com",
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
		},
		"no scope handler": {
			Token: oauthapi.OAuthAccessToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength"},
				ClientName:  "myclient",
				UserName:    "myusername",
				UserUID:     "myuseruid",
				Scopes:      []string{"invalid"},
				RedirectURI: "https://authn.mycluster.com",
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
		},
		"bad scope": {
			Token: oauthapi.OAuthAccessToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength"},
				ClientName:  "myclient",
				UserName:    "myusername",
				UserUID:     "myuseruid",
				Scopes:      []string{"user:dne"},
				RedirectURI: "https://authn.mycluster.com",
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
		},
		"negative timeout": {
			Token: oauthapi.OAuthAccessToken{
				ObjectMeta:               metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength"},
				ClientName:               "myclient",
				UserName:                 "myusername",
				UserUID:                  "myuseruid",
				InactivityTimeoutSeconds: -1,
				Scopes:                   []string{"user:check-access"},
				RedirectURI:              "https://authn.mycluster.com",
			},
			T: field.ErrorTypeInvalid,
			F: "inactivityTimeoutSeconds",
		},
		"negative expiresIn": {
			Token: oauthapi.OAuthAccessToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength"},
				ClientName:  "myclient",
				UserName:    "myusername",
				UserUID:     "myuseruid",
				ExpiresIn:   -1,
				Scopes:      []string{"user:check-access"},
				RedirectURI: "https://authn.mycluster.com",
			},
			T: field.ErrorTypeInvalid,
			F: "expiresIn",
		},
	}
	for k, v := range errorCases {
		errs := ValidateAccessToken(&v.Token)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.Token)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateAuthorizeTokens(t *testing.T) {
	errs := ValidateAuthorizeToken(&oauthapi.OAuthAuthorizeToken{
		ObjectMeta:  metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
		ClientName:  "myclient",
		ExpiresIn:   86400,
		UserName:    "myusername",
		UserUID:     "myuseruid",
		Scopes:      []string{`user:info`},
		RedirectURI: "https://authz.mycluster.com",
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		Token oauthapi.OAuthAuthorizeToken
		T     field.ErrorType
		F     string
	}{
		"zero-length name": {
			Token: oauthapi.OAuthAuthorizeToken{
				ClientName:  "myclient",
				ExpiresIn:   86400,
				UserName:    "myusername",
				UserUID:     "myuseruid",
				Scopes:      []string{"user:full"},
				RedirectURI: "https://authz.mycluster.com",
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"zero-length client name": {
			Token: oauthapi.OAuthAuthorizeToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				UserName:    "myusername",
				ExpiresIn:   86400,
				UserUID:     "myuseruid",
				Scopes:      []string{"user:full"},
				RedirectURI: "https://authz.mycluster.com",
			},
			T: field.ErrorTypeRequired,
			F: "clientName",
		},
		"zero-length user name": {
			Token: oauthapi.OAuthAuthorizeToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName:  "myclient",
				ExpiresIn:   86400,
				UserUID:     "myuseruid",
				Scopes:      []string{"user:full"},
				RedirectURI: "https://authz.mycluster.com",
			},
			T: field.ErrorTypeRequired,
			F: "userName",
		},
		"zero-length user uid": {
			Token: oauthapi.OAuthAuthorizeToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName:  "myclient",
				ExpiresIn:   86400,
				UserName:    "myusername",
				Scopes:      []string{"user:full"},
				RedirectURI: "https://authz.mycluster.com",
			},
			T: field.ErrorTypeRequired,
			F: "userUID",
		},
		"empty redirect uri": {
			Token: oauthapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "myperfectTokenWithFullScopeAndStuff"},
				ClientName: "myclient",
				ExpiresIn:  86400,
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{"user:full"},
			},
			T: field.ErrorTypeInvalid,
			F: "redirectURI",
		},
		"empty scopes": {
			Token: oauthapi.OAuthAuthorizeToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "myperfectTokenWithFullScopeAndStuff"},
				ClientName:  "myclient",
				ExpiresIn:   86400,
				UserName:    "myusername",
				UserUID:     "myuseruid",
				RedirectURI: "https://authz.mycluster.com",
			},
			T: field.ErrorTypeRequired,
			F: "scopes",
		},
		"disallowed namespace": {
			Token: oauthapi.OAuthAuthorizeToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength", Namespace: "foo"},
				ClientName:  "myclient",
				ExpiresIn:   86400,
				UserName:    "myusername",
				UserUID:     "myuseruid",
				Scopes:      []string{"user:check-access"},
				RedirectURI: "https://authz.mycluster.com",
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
		},
		"no scope handler": {
			Token: oauthapi.OAuthAuthorizeToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName:  "myclient",
				ExpiresIn:   86400,
				UserName:    "myusername",
				UserUID:     "myuseruid",
				Scopes:      []string{"invalid"},
				RedirectURI: "https://authz.mycluster.com",
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
		},
		"bad scope": {
			Token: oauthapi.OAuthAuthorizeToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName:  "myclient",
				ExpiresIn:   86400,
				UserName:    "myusername",
				UserUID:     "myuseruid",
				Scopes:      []string{"user:dne"},
				RedirectURI: "https://authz.mycluster.com",
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
		},
		"illegal character": {
			Token: oauthapi.OAuthAuthorizeToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName:  "myclient",
				ExpiresIn:   86400,
				UserName:    "myusername",
				UserUID:     "myuseruid",
				Scopes:      []string{`role:asdf":foo`},
				RedirectURI: "https://authz.mycluster.com",
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
		},
		"zero expiresIn": {
			Token: oauthapi.OAuthAuthorizeToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName:  "myclient",
				UserName:    "myusername",
				UserUID:     "myuseruid",
				Scopes:      []string{"user:check-access"},
				RedirectURI: "https://authz.mycluster.com",
			},
			T: field.ErrorTypeInvalid,
			F: "expiresIn",
		},
		"negative expiresIn": {
			Token: oauthapi.OAuthAuthorizeToken{
				ObjectMeta:  metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName:  "myclient",
				ExpiresIn:   -1,
				UserName:    "myusername",
				UserUID:     "myuseruid",
				Scopes:      []string{"user:check-access"},
				RedirectURI: "https://authz.mycluster.com",
			},
			T: field.ErrorTypeInvalid,
			F: "expiresIn",
		},
	}
	for k, v := range errorCases {
		errs := ValidateAuthorizeToken(&v.Token)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.Token)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateAccessTokensUpdate(t *testing.T) {
	valid := &oauthapi.OAuthAccessToken{
		ObjectMeta:               metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength", ResourceVersion: "1"},
		ClientName:               "myclient",
		UserName:                 "myusername",
		UserUID:                  "myuseruid",
		InactivityTimeoutSeconds: 300,
	}
	validNoTimeout := &oauthapi.OAuthAccessToken{
		ObjectMeta:               metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength", ResourceVersion: "1"},
		ClientName:               "myclient",
		UserName:                 "myusername",
		UserUID:                  "myuseruid",
		InactivityTimeoutSeconds: 0,
	}
	errs := ValidateAccessTokenUpdate(valid, valid)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}
	errs = ValidateAccessTokenUpdate(validNoTimeout, validNoTimeout)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		Token  oauthapi.OAuthAccessToken
		Change func(*oauthapi.OAuthAccessToken)
		T      field.ErrorType
		F      string
	}{
		"change name": {
			Token: *valid,
			Change: func(obj *oauthapi.OAuthAccessToken) {
				obj.Name = ""
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.name",
		},
		"change userName": {
			Token: *valid,
			Change: func(obj *oauthapi.OAuthAccessToken) {
				obj.UserName = ""
			},
			T: field.ErrorTypeInvalid,
			F: "[]",
		},
		"change InactivityTimeoutSeconds to smaller value": {
			Token: *valid,
			Change: func(obj *oauthapi.OAuthAccessToken) {
				obj.InactivityTimeoutSeconds = 299
			},
			T: field.ErrorTypeInvalid,
			F: "inactivityTimeoutSeconds",
		},
		"change InactivityTimeoutSeconds to negative value": {
			Token: *valid,
			Change: func(obj *oauthapi.OAuthAccessToken) {
				obj.InactivityTimeoutSeconds = -1
			},
			T: field.ErrorTypeInvalid,
			F: "inactivityTimeoutSeconds",
		},
		"change InactivityTimeoutSeconds from 0 value": {
			Token: *validNoTimeout,
			Change: func(obj *oauthapi.OAuthAccessToken) {
				obj.InactivityTimeoutSeconds = MinimumInactivityTimeoutSeconds
			},
			T: field.ErrorTypeInvalid,
			F: "inactivityTimeoutSeconds",
		},
	}
	for k, v := range errorCases {
		newToken := v.Token.DeepCopy()
		v.Change(newToken)
		errs := ValidateAccessTokenUpdate(newToken, &v.Token)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.Token)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateAuthorizeTokensUpdate(t *testing.T) {
	valid := &oauthapi.OAuthAuthorizeToken{
		ObjectMeta: metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength", ResourceVersion: "1"},
		ClientName: "myclient",
		UserName:   "myusername",
		UserUID:    "myuseruid",
		Scopes:     []string{`user:info`},
	}
	errs := ValidateAuthorizeTokenUpdate(valid, valid)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		Token  oauthapi.OAuthAuthorizeToken
		Change func(*oauthapi.OAuthAuthorizeToken)
		T      field.ErrorType
		F      string
	}{
		"change name": {
			Token: *valid,
			Change: func(obj *oauthapi.OAuthAuthorizeToken) {
				obj.Name = ""
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.name",
		},
		"change userUID": {
			Token: *valid,
			Change: func(obj *oauthapi.OAuthAuthorizeToken) {
				obj.UserUID = ""
			},
			T: field.ErrorTypeInvalid,
			F: "[]",
		},
	}
	for k, v := range errorCases {
		newToken := v.Token.DeepCopy()
		v.Change(newToken)
		errs := ValidateAuthorizeTokenUpdate(newToken, &v.Token)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.Token)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}
