package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/RangelReale/osin"
	"github.com/openshift/origin/pkg/oauthserver/osinserver"
)

func TestAuthorizeAuthenticator(t *testing.T) {
	_ = osinserver.AuthorizeHandler(&AuthorizeAuthenticator{})
}

func TestAuthenticator(t *testing.T) {
	testCases := map[osin.AccessRequestType]struct {
		ExpectedAuthorized bool
		ExpectedError      bool
	}{
		osin.AUTHORIZATION_CODE: {true, false},
		osin.REFRESH_TOKEN:      {true, false},
		osin.PASSWORD:           {false, false},
		osin.ASSERTION:          {false, false},
		osin.CLIENT_CREDENTIALS: {false, false},
		osin.IMPLICIT:           {false, false},
	}

	for requestType, testCase := range testCases {
		deny := osinserver.AccessHandler(NewAccessAuthenticator(Deny, Deny, Deny))
		req := &osin.AccessRequest{Type: requestType}
		w := httptest.NewRecorder()
		err := deny.HandleAccess(req, w)
		if testCase.ExpectedError && err == nil {
			t.Fatalf("%s: Expected error, got success", requestType)
		}
		if !testCase.ExpectedError && err != nil {
			t.Fatalf("%s: Unexpected error: %s", requestType, err)
		}
		if req.Authorized != testCase.ExpectedAuthorized {
			t.Fatalf("%s: Expected Authorized=%t, got Authorized=%t", requestType, testCase.ExpectedAuthorized, req.Authorized)
		}
	}
}

func TestDenyPassword(t *testing.T) {
	user, ok, err := Deny.AuthenticatePassword("", "")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Unexpected success")
	}
	if user != nil {
		t.Fatalf("Unexpected user info: %v", user)
	}
}

func TestDenyAssertion(t *testing.T) {
	user, ok, err := Deny.AuthenticateAssertion("", "")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Unexpected success")
	}
	if user != nil {
		t.Fatalf("Unexpected user info: %v", user)
	}
}

func TestDenyClient(t *testing.T) {
	user, ok, err := Deny.AuthenticateClient(nil)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("Unexpected success")
	}
	if user != nil {
		t.Fatalf("Unexpected user info: %v", user)
	}
}

func TestAllowPassword(t *testing.T) {
	user, ok, err := Allow.AuthenticatePassword("", "")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Unexpected failure")
	}
	if user != nil {
		t.Fatalf("Unexpected user info: %v", user)
	}
}

func TestAllowAssertion(t *testing.T) {
	user, ok, err := Allow.AuthenticateAssertion("", "")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Unexpected failure")
	}
	if user != nil {
		t.Fatalf("Unexpected user info: %v", user)
	}
}

func TestAllowClient(t *testing.T) {
	user, ok, err := Allow.AuthenticateClient(nil)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("Unexpected failure")
	}
	if user != nil {
		t.Fatalf("Unexpected user info: %v", user)
	}
}
