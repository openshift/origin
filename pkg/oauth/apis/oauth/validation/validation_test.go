package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	oapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

func TestValidateRedirectURI(t *testing.T) {
	allowed := []string{
		// Empty allowed
		"",

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
		"https://server:port",

		// With or without paths, with or without trailing slashes
		"https://server:port/",
		"https://server:port/path-segment",
		"https://server:port/path-segment/",

		// Things that are close to disallowed path segments
		"https://server:port/...",
		"https://server:port/.../",
		"https://server:port/path-segment/...",
		"https://server:port/path-segment/path.",
		"https://server:port/path-segment/path./",

		// Double slashes
		"https://server:port/path-segment//path",

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
		// invalid URL
		"://server:port/",

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
	errs := ValidateClientAuthorization(&oapi.OAuthClientAuthorization{
		ObjectMeta: metav1.ObjectMeta{Name: "myusername:myclientname"},
		ClientName: "myclientname",
		UserName:   "myusername",
		UserUID:    "myuseruid",
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A oapi.OAuthClientAuthorization
		T field.ErrorType
		F string
	}{
		"zero-length name": {
			A: oapi.OAuthClientAuthorization{
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid name": {
			A: oapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "anotheruser:anotherclient"},
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.name",
		},
		"disallowed namespace": {
			A: oapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "myusername:myclientname", Namespace: "foo"},
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
		},
		"no scope handler": {
			A: oapi.OAuthClientAuthorization{
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
			A: oapi.OAuthClientAuthorization{
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
	errs := ValidateClient(&oapi.OAuthClient{
		ObjectMeta: metav1.ObjectMeta{Name: "client-name"},
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	var badTimeout int32 = MinimumInactivityTimeoutSeconds - 1
	var negTimeout int32 = -1

	errorCases := map[string]struct {
		Client oapi.OAuthClient
		T      field.ErrorType
		F      string
	}{
		"zero-length name": {
			Client: oapi.OAuthClient{},
			T:      field.ErrorTypeRequired,
			F:      "metadata.name",
		},
		"disallowed namespace": {
			Client: oapi.OAuthClient{ObjectMeta: metav1.ObjectMeta{Name: "name", Namespace: "foo"}},
			T:      field.ErrorTypeForbidden,
			F:      "metadata.namespace",
		},
		"literal must have value": {
			Client: oapi.OAuthClient{
				ObjectMeta:        metav1.ObjectMeta{Name: "client-name"},
				ScopeRestrictions: []oapi.ScopeRestriction{{ExactValues: []string{""}}},
			},
			T: field.ErrorTypeInvalid,
			F: "scopeRestrictions[0].literals[0]",
		},
		"must have role names": {
			Client: oapi.OAuthClient{
				ObjectMeta: metav1.ObjectMeta{Name: "client-name"},
				ScopeRestrictions: []oapi.ScopeRestriction{
					{
						ClusterRole: &oapi.ClusterRoleScopeRestriction{Namespaces: []string{"b"}},
					},
				},
			},
			T: field.ErrorTypeRequired,
			F: "scopeRestrictions[0].clusterRole.roleNames",
		},
		"must have namespaces": {
			Client: oapi.OAuthClient{
				ObjectMeta: metav1.ObjectMeta{Name: "client-name"},
				ScopeRestrictions: []oapi.ScopeRestriction{
					{
						ClusterRole: &oapi.ClusterRoleScopeRestriction{RoleNames: []string{"a"}},
					},
				},
			},
			T: field.ErrorTypeRequired,
			F: "scopeRestrictions[0].clusterRole.namespaces",
		},
		"minimum timeout value": {
			Client: oapi.OAuthClient{
				ObjectMeta:                          metav1.ObjectMeta{Name: "client-name"},
				AccessTokenInactivityTimeoutSeconds: &badTimeout,
			},
			T: field.ErrorTypeInvalid,
			F: "accessTokenInactivityTimeoutSeconds",
		},
		"negative timeout value": {
			Client: oapi.OAuthClient{
				ObjectMeta:                          metav1.ObjectMeta{Name: "client-name"},
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
	errs := ValidateAccessToken(&oapi.OAuthAccessToken{
		ObjectMeta: metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength"},
		ClientName: "myclient",
		UserName:   "myusername",
		UserUID:    "myuseruid",
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		Token oapi.OAuthAccessToken
		T     field.ErrorType
		F     string
	}{
		"zero-length name": {
			Token: oapi.OAuthAccessToken{
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"disallowed namespace": {
			Token: oapi.OAuthAccessToken{
				ObjectMeta: metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength", Namespace: "foo"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
		},
		"no scope handler": {
			Token: oapi.OAuthAccessToken{
				ObjectMeta: metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{"invalid"},
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
		},
		"bad scope": {
			Token: oapi.OAuthAccessToken{
				ObjectMeta: metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{"user:dne"},
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
		},
		"negative timeout": {
			Token: oapi.OAuthAccessToken{
				ObjectMeta:               metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength"},
				ClientName:               "myclient",
				UserName:                 "myusername",
				UserUID:                  "myuseruid",
				InactivityTimeoutSeconds: -1,
			},
			T: field.ErrorTypeInvalid,
			F: "inactivityTimeoutSeconds",
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
	errs := ValidateAuthorizeToken(&oapi.OAuthAuthorizeToken{
		ObjectMeta: metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
		ClientName: "myclient",
		UserName:   "myusername",
		UserUID:    "myuseruid",
		Scopes:     []string{`user:info`},
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		Token oapi.OAuthAuthorizeToken
		T     field.ErrorType
		F     string
	}{
		"zero-length name": {
			Token: oapi.OAuthAuthorizeToken{
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeRequired,
			F: "metadata.name",
		},
		"zero-length client name": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeRequired,
			F: "clientName",
		},
		"zero-length user name": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName: "myclient",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeRequired,
			F: "userName",
		},
		"zero-length user uid": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName: "myclient",
				UserName:   "myusername",
			},
			T: field.ErrorTypeRequired,
			F: "userUID",
		},
		"disallowed namespace": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength", Namespace: "foo"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
		},
		"no scope handler": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{"invalid"},
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
		},
		"bad scope": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{"user:dne"},
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
		},
		"illegal character": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
				Scopes:     []string{`role:asdf":foo`},
			},
			T: field.ErrorTypeInvalid,
			F: "scopes[0]",
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
	valid := &oapi.OAuthAccessToken{
		ObjectMeta:               metav1.ObjectMeta{Name: "accessTokenNameWithMinimumLength", ResourceVersion: "1"},
		ClientName:               "myclient",
		UserName:                 "myusername",
		UserUID:                  "myuseruid",
		InactivityTimeoutSeconds: 300,
	}
	validNoTimeout := &oapi.OAuthAccessToken{
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
		Token  oapi.OAuthAccessToken
		Change func(*oapi.OAuthAccessToken)
		T      field.ErrorType
		F      string
	}{
		"change name": {
			Token: *valid,
			Change: func(obj *oapi.OAuthAccessToken) {
				obj.Name = ""
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.name",
		},
		"change userName": {
			Token: *valid,
			Change: func(obj *oapi.OAuthAccessToken) {
				obj.UserName = ""
			},
			T: field.ErrorTypeInvalid,
			F: "[]",
		},
		"change InactivityTimeoutSeconds to smaller value": {
			Token: *valid,
			Change: func(obj *oapi.OAuthAccessToken) {
				obj.InactivityTimeoutSeconds = 299
			},
			T: field.ErrorTypeInvalid,
			F: "inactivityTimeoutSeconds",
		},
		"change InactivityTimeoutSeconds to negative value": {
			Token: *valid,
			Change: func(obj *oapi.OAuthAccessToken) {
				obj.InactivityTimeoutSeconds = -1
			},
			T: field.ErrorTypeInvalid,
			F: "inactivityTimeoutSeconds",
		},
		"change InactivityTimeoutSeconds from 0 value": {
			Token: *validNoTimeout,
			Change: func(obj *oapi.OAuthAccessToken) {
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
	valid := &oapi.OAuthAuthorizeToken{
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
		Token  oapi.OAuthAuthorizeToken
		Change func(*oapi.OAuthAuthorizeToken)
		T      field.ErrorType
		F      string
	}{
		"change name": {
			Token: *valid,
			Change: func(obj *oapi.OAuthAuthorizeToken) {
				obj.Name = ""
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.name",
		},
		"change userUID": {
			Token: *valid,
			Change: func(obj *oapi.OAuthAuthorizeToken) {
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
