package external

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RangelReale/osincli"
	"github.com/openshift/oauth-server/pkg/oauth/handlers"
	"github.com/openshift/oauth-server/pkg/server/csrf"
	"k8s.io/apiserver/pkg/authentication/user"
)

func TestHandler(t *testing.T) {
	redirectors := new(handlers.AuthenticationRedirectors)
	redirectors.Add("handler", &Handler{})
	_ = handlers.NewUnionAuthenticationHandler(nil, redirectors, nil, nil)
}

func TestRedirectingStateValidCSRF(t *testing.T) {
	fakeCSRF := &csrf.FakeCSRF{
		Token: "xyz",
	}
	redirectingState := CSRFRedirectingState(fakeCSRF)

	req, _ := http.NewRequest("GET", "http://www.example.com", nil)
	state, err := redirectingState.Generate(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	// Make sure the state verifies
	req2, _ := http.NewRequest("GET", "http://www.example.com/callback", nil)
	ok, err := redirectingState.Check(state, req2)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if !ok {
		t.Fatalf("Unexpected invalid state")
	}
}

func TestRedirectingStateInvalidCSRF(t *testing.T) {
	fakeCSRF := &csrf.FakeCSRF{
		Token: "xyz",
	}
	redirectingState := CSRFRedirectingState(fakeCSRF)

	req, _ := http.NewRequest("GET", "http://www.example.com", nil)
	state, err := redirectingState.Generate(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	req2, _ := http.NewRequest("GET", "http://www.example.com/callback", nil)

	// Initial check passes as valid with no error
	if ok, err := redirectingState.Check(state, req2); !ok || err != nil {
		t.Fatalf("Expected valid and no error, got: %v %v", ok, err)
	}

	// Change the CSRF validator so it returns invalid and an error
	fakeCSRF.Token = "abc"
	if ok, err := redirectingState.Check(state, req2); ok || err == nil {
		t.Fatalf("Expected invalid and error, got: %v %v", ok, err)
	}
}

func TestRedirectingStateSuccess(t *testing.T) {
	originalURL := "http://www.example.com"

	fakeCSRF := &csrf.FakeCSRF{
		Token: "xyz",
	}
	redirectingState := CSRFRedirectingState(fakeCSRF)

	req, _ := http.NewRequest("GET", originalURL, nil)
	state, err := redirectingState.Generate(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	req2, _ := http.NewRequest("GET", "http://www.example.com/callback", nil)
	recorder := httptest.NewRecorder()
	user := &user.DefaultInfo{}

	handled, err := redirectingState.AuthenticationSucceeded(user, state, recorder, req2)
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if !handled {
		t.Errorf("Expected handled request")
	}
	if recorder.Header().Get("Location") != originalURL {
		t.Errorf("Expected redirect to %s, got %#v", originalURL, recorder.Header())
	}
}

func TestRedirectingStateOAuthError(t *testing.T) {
	originalURL := "http://www.example.com"
	expectedURL := "http://www.example.com?error=access_denied"

	fakeCSRF := &csrf.FakeCSRF{
		Token: "xyz",
	}
	redirectingState := CSRFRedirectingState(fakeCSRF)

	req, _ := http.NewRequest("GET", originalURL, nil)
	state, err := redirectingState.Generate(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	req2, _ := http.NewRequest("GET", "http://www.example.com/callback", nil)
	recorder := httptest.NewRecorder()
	osinErr := &osincli.Error{
		Id:    "access_denied",
		State: state,
	}

	handled, err := redirectingState.AuthenticationError(osinErr, recorder, req2)
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if !handled {
		t.Errorf("Expected handled request")
	}
	if recorder.Header().Get("Location") != expectedURL {
		t.Errorf("Expected redirect to %s, got %#v", expectedURL, recorder.Header())
	}
}

func TestRedirectingStateError(t *testing.T) {
	fakeCSRF := &csrf.FakeCSRF{
		Token: "xyz",
	}
	redirectingState := CSRFRedirectingState(fakeCSRF)

	req2, _ := http.NewRequest("GET", "http://www.example.com/callback", nil)
	recorder := httptest.NewRecorder()
	inErr := errors.New("test")

	handled, err := redirectingState.AuthenticationError(inErr, recorder, req2)
	if handled {
		t.Errorf("Expected unhandled request")
	}
	if err != inErr {
		t.Errorf("Expected original error back, got %#v", err)
	}
}
