package validation

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	oapi "github.com/openshift/origin/pkg/oauth/api"
)

func TestValidateClientAuthorization(t *testing.T) {
	errs := ValidateClientAuthorization(&oapi.ClientAuthorization{
		ObjectMeta: api.ObjectMeta{Name: "authName"},
		ClientName: "myclientname",
		UserName:   "myusername",
		UserUID:    "myuseruid",
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A oapi.ClientAuthorization
		T errors.ValidationErrorType
		F string
	}{
		"zero-length name": {
			A: oapi.ClientAuthorization{
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: errors.ValidationErrorTypeRequired,
			F: "name",
		},
		"disallowed namespace": {
			A: oapi.ClientAuthorization{
				ObjectMeta: api.ObjectMeta{Name: "name", Namespace: "foo"},
				ClientName: "myclientname",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: errors.ValidationErrorTypeInvalid,
			F: "namespace",
		},
	}
	for k, v := range errorCases {
		errs := ValidateClientAuthorization(&v.A)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].(*errors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*errors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateClient(t *testing.T) {
	errs := ValidateClient(&oapi.Client{
		ObjectMeta: api.ObjectMeta{Name: "clientName"},
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		Client oapi.Client
		T      errors.ValidationErrorType
		F      string
	}{
		"zero-length name": {
			Client: oapi.Client{},
			T:      errors.ValidationErrorTypeRequired,
			F:      "name",
		},
		"disallowed namespace": {
			Client: oapi.Client{ObjectMeta: api.ObjectMeta{Name: "name", Namespace: "foo"}},
			T:      errors.ValidationErrorTypeInvalid,
			F:      "namespace",
		},
	}
	for k, v := range errorCases {
		errs := ValidateClient(&v.Client)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.Client)
			continue
		}
		for i := range errs {
			if errs[i].(*errors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*errors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateAccessTokens(t *testing.T) {
	errs := ValidateAccessToken(&oapi.AccessToken{
		ObjectMeta: api.ObjectMeta{Name: "accessTokenName"},
		ClientName: "myclient",
		UserName:   "myusername",
		UserUID:    "myuseruid",
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		Token oapi.AccessToken
		T     errors.ValidationErrorType
		F     string
	}{
		"zero-length name": {
			Token: oapi.AccessToken{
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: errors.ValidationErrorTypeRequired,
			F: "name",
		},
		"disallowed namespace": {
			Token: oapi.AccessToken{
				ObjectMeta: api.ObjectMeta{Name: "name", Namespace: "foo"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: errors.ValidationErrorTypeInvalid,
			F: "namespace",
		},
	}
	for k, v := range errorCases {
		errs := ValidateAccessToken(&v.Token)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.Token)
			continue
		}
		for i := range errs {
			if errs[i].(*errors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*errors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateAuthorizeTokens(t *testing.T) {
	errs := ValidateAuthorizeToken(&oapi.AuthorizeToken{
		ObjectMeta: api.ObjectMeta{Name: "authorizeTokenName"},
		ClientName: "myclient",
		UserName:   "myusername",
		UserUID:    "myuseruid",
	})
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		Token oapi.AuthorizeToken
		T     errors.ValidationErrorType
		F     string
	}{
		"zero-length name": {
			Token: oapi.AuthorizeToken{
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: errors.ValidationErrorTypeRequired,
			F: "name",
		},
		"zero-length client name": {
			Token: oapi.AuthorizeToken{
				ObjectMeta: api.ObjectMeta{Name: "authorizeTokenName"},
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: errors.ValidationErrorTypeRequired,
			F: "clientname",
		},
		"zero-length user name": {
			Token: oapi.AuthorizeToken{
				ObjectMeta: api.ObjectMeta{Name: "authorizeTokenName"},
				ClientName: "myclient",
				UserUID:    "myuseruid",
			},
			T: errors.ValidationErrorTypeRequired,
			F: "username",
		},
		"zero-length user uid": {
			Token: oapi.AuthorizeToken{
				ObjectMeta: api.ObjectMeta{Name: "authorizeTokenName"},
				ClientName: "myclient",
				UserName:   "myusername",
			},
			T: errors.ValidationErrorTypeRequired,
			F: "useruid",
		},
		"disallowed namespace": {
			Token: oapi.AuthorizeToken{
				ObjectMeta: api.ObjectMeta{Name: "name", Namespace: "foo"},
				ClientName: "myclient",
				UserName:   "myusername",
				UserUID:    "myuseruid",
			},
			T: errors.ValidationErrorTypeInvalid,
			F: "namespace",
		},
	}
	for k, v := range errorCases {
		errs := ValidateAuthorizeToken(&v.Token)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.Token)
			continue
		}
		for i := range errs {
			if errs[i].(*errors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*errors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}
