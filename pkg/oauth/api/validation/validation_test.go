package validation

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/validation/field"

	oapi "github.com/openshift/origin/pkg/oauth/api"
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
		ObjectMeta: api.ObjectMeta{Name: "myusername:myclientname"},
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
				ObjectMeta: api.ObjectMeta{Name: "anotheruser:anotherclient"},
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeInvalid,
			F: "metadata.name",
		},
		"disallowed namespace": {
			A: oapi.OAuthClientAuthorization{
				ObjectMeta: api.ObjectMeta{Name: "myusername:myclientname", Namespace: "foo"},
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
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
		ObjectMeta: api.ObjectMeta{Name: "client-name"},
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

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
			Client: oapi.OAuthClient{ObjectMeta: api.ObjectMeta{Name: "name", Namespace: "foo"}},
			T:      field.ErrorTypeForbidden,
			F:      "metadata.namespace",
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
		ObjectMeta: api.ObjectMeta{Name: "accessTokenNameWithMinimumLength"},
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
				ObjectMeta: api.ObjectMeta{Name: "accessTokenNameWithMinimumLength", Namespace: "foo"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
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
		ObjectMeta: api.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
		ClientName: "myclient",
		UserName:   "myusername",
		UserUID:    "myuseruid",
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
				ObjectMeta: api.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeRequired,
			F: "clientName",
		},
		"zero-length user name": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: api.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName: "myclient",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeRequired,
			F: "userName",
		},
		"zero-length user uid": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: api.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName: "myclient",
				UserName:   "myusername",
			},
			T: field.ErrorTypeRequired,
			F: "userUID",
		},
		"disallowed namespace": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: api.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength", Namespace: "foo"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: field.ErrorTypeForbidden,
			F: "metadata.namespace",
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
