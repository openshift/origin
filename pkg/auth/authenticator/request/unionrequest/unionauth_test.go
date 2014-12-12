package unionrequest

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

type mockAuthRequestHandler struct {
	returnUser      authapi.UserInfo
	isAuthenticated bool
	err             error
}

func (mock *mockAuthRequestHandler) AuthenticateRequest(req *http.Request) (authapi.UserInfo, bool, error) {
	return mock.returnUser, mock.isAuthenticated, mock.err
}

func TestAuthenticateRequestSecondPasses(t *testing.T) {
	handler1 := &mockAuthRequestHandler{}
	handler2 := &mockAuthRequestHandler{isAuthenticated: true}
	authRequestHandler := NewUnionAuthentication([]authenticator.Request{handler1, handler2})
	req, _ := http.NewRequest("GET", "http://example.org", nil)

	_, isAuthenticated, err := authRequestHandler.AuthenticateRequest(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !isAuthenticated {
		t.Errorf("Unexpectedly unauthenticated: %v", isAuthenticated)
	}
}

func TestAuthenticateRequestSuppressUnnecessaryErrors(t *testing.T) {
	handler1 := &mockAuthRequestHandler{err: errors.New("first")}
	handler2 := &mockAuthRequestHandler{isAuthenticated: true}
	authRequestHandler := NewUnionAuthentication([]authenticator.Request{handler1, handler2})
	req, _ := http.NewRequest("GET", "http://example.org", nil)

	_, isAuthenticated, err := authRequestHandler.AuthenticateRequest(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !isAuthenticated {
		t.Errorf("Unexpectedly unauthenticated: %v", isAuthenticated)
	}
}

func TestAuthenticateRequestNonePass(t *testing.T) {
	handler1 := &mockAuthRequestHandler{}
	handler2 := &mockAuthRequestHandler{}
	authRequestHandler := NewUnionAuthentication([]authenticator.Request{handler1, handler2})
	req, _ := http.NewRequest("GET", "http://example.org", nil)

	_, isAuthenticated, err := authRequestHandler.AuthenticateRequest(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if isAuthenticated {
		t.Errorf("Unexpectedly authenticated: %v", isAuthenticated)
	}
}

func TestAuthenticateRequestAdditiveErrors(t *testing.T) {
	handler1 := &mockAuthRequestHandler{err: errors.New("first")}
	handler2 := &mockAuthRequestHandler{err: errors.New("second")}
	authRequestHandler := NewUnionAuthentication([]authenticator.Request{handler1, handler2})
	req, _ := http.NewRequest("GET", "http://example.org", nil)

	_, isAuthenticated, err := authRequestHandler.AuthenticateRequest(req)
	if err == nil {
		t.Errorf("Expected an error")
	}
	if !strings.Contains(err.Error(), "first") {
		t.Errorf("Expected error containing %v, got %v", "first", err)
	}
	if !strings.Contains(err.Error(), "second") {
		t.Errorf("Expected error containing %v, got %v", "second", err)
	}
	if isAuthenticated {
		t.Errorf("Unexpectedly authenticated: %v", isAuthenticated)
	}
}
