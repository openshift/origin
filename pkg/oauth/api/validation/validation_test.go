package validation

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	oapi "github.com/openshift/origin/pkg/oauth/api"
)

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
		T fielderrors.ValidationErrorType
		F string
	}{
		"zero-length name": {
			A: oapi.OAuthClientAuthorization{
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "metadata.name",
		},
		"invalid name": {
			A: oapi.OAuthClientAuthorization{
				ObjectMeta: api.ObjectMeta{Name: "anotheruser:anotherclient"},
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "metadata.name",
		},
		"disallowed namespace": {
			A: oapi.OAuthClientAuthorization{
				ObjectMeta: api.ObjectMeta{Name: "myusername:myclientname", Namespace: "foo"},
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: fielderrors.ValidationErrorTypeInvalid,
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
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
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
		T      fielderrors.ValidationErrorType
		F      string
	}{
		"zero-length name": {
			Client: oapi.OAuthClient{},
			T:      fielderrors.ValidationErrorTypeRequired,
			F:      "metadata.name",
		},
		"disallowed namespace": {
			Client: oapi.OAuthClient{ObjectMeta: api.ObjectMeta{Name: "name", Namespace: "foo"}},
			T:      fielderrors.ValidationErrorTypeInvalid,
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
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
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
		T     fielderrors.ValidationErrorType
		F     string
	}{
		"zero-length name": {
			Token: oapi.OAuthAccessToken{
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "metadata.name",
		},
		"disallowed namespace": {
			Token: oapi.OAuthAccessToken{
				ObjectMeta: api.ObjectMeta{Name: "accessTokenNameWithMinimumLength", Namespace: "foo"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: fielderrors.ValidationErrorTypeInvalid,
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
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
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
		T     fielderrors.ValidationErrorType
		F     string
	}{
		"zero-length name": {
			Token: oapi.OAuthAuthorizeToken{
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "metadata.name",
		},
		"zero-length client name": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: api.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "clientName",
		},
		"zero-length user name": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: api.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName: "myclient",
				UserUID:    "myuseruid",
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "userName",
		},
		"zero-length user uid": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: api.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength"},
				ClientName: "myclient",
				UserName:   "myusername",
			},
			T: fielderrors.ValidationErrorTypeRequired,
			F: "userUID",
		},
		"disallowed namespace": {
			Token: oapi.OAuthAuthorizeToken{
				ObjectMeta: api.ObjectMeta{Name: "authorizeTokenNameWithMinimumLength", Namespace: "foo"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: fielderrors.ValidationErrorTypeInvalid,
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
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}
