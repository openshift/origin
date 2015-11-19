package basicauthrequest

import (
	"net/http"
	"testing"

	"k8s.io/kubernetes/pkg/auth/user"
)

const (
	Username          = "frightened_donut"
	Password          = "don't eat me!"
	ValidBase64String = "VGhpc0lzVmFsaWQK" // base64 -- ThisIsValid ctrl+d
)

type mockPasswordAuthenticator struct {
	returnUser      user.Info
	isAuthenticated bool
	err             error
	passedUser      string
	passedPassword  string
}

func (mock *mockPasswordAuthenticator) AuthenticatePassword(username, password string) (user.Info, bool, error) {
	mock.passedUser = username
	mock.passedPassword = password

	return mock.returnUser, mock.isAuthenticated, mock.err
}

func TestAuthenticateRequestValid(t *testing.T) {
	passwordAuthenticator := &mockPasswordAuthenticator{}
	authRequestHandler := NewBasicAuthAuthentication(passwordAuthenticator, true)
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	req.SetBasicAuth(Username, Password)

	_, _, _ = authRequestHandler.AuthenticateRequest(req)
	if passwordAuthenticator.passedUser != Username {
		t.Errorf("Expected %v, got %v", Username, passwordAuthenticator.passedUser)
	}
	if passwordAuthenticator.passedPassword != Password {
		t.Errorf("Expected %v, got %v", Password, passwordAuthenticator.passedPassword)
	}
}

func TestAuthenticateRequestInvalid(t *testing.T) {
	const (
		ExpectedError = "No valid base64 data in basic auth scheme found"
	)
	passwordAuthenticator := &mockPasswordAuthenticator{isAuthenticated: true}
	authRequestHandler := NewBasicAuthAuthentication(passwordAuthenticator, true)
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	req.Header.Add("Authorization", "Basic invalid:string")

	userInfo, authenticated, err := authRequestHandler.AuthenticateRequest(req)
	if err == nil {
		t.Errorf("Expected error: %v", ExpectedError)
	}
	if err.Error() != ExpectedError {
		t.Errorf("Expected %v, got %v", ExpectedError, err)
	}
	if userInfo != nil {
		t.Errorf("Unexpected user: %v", userInfo)
	}
	if authenticated {
		t.Errorf("Unexpectedly authenticated: %v", authenticated)
	}
}

func TestGetBasicAuthInfo(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	req.SetBasicAuth(Username, Password)

	username, password, hasBasicAuth, err := getBasicAuthInfo(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !hasBasicAuth {
		t.Errorf("Expected hasBasicAuth")
	}
	if username != Username {
		t.Errorf("Expected %v, got %v", Username, username)
	}
	if password != Password {
		t.Errorf("Expected %v, got %v", Password, password)
	}
}

func TestGetBasicAuthInfoNoHeader(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.org", nil)

	username, password, hasBasicAuth, err := getBasicAuthInfo(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if hasBasicAuth {
		t.Errorf("Expected hasBasicAuth to be false")
	}
	if len(username) != 0 {
		t.Errorf("Unexpected username: %v", username)
	}
	if len(password) != 0 {
		t.Errorf("Unexpected password: %v", password)
	}
}

func TestGetBasicAuthInfoNotBasicHeader(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	req.Header.Add("Authorization", "notbasic")

	username, password, hasBasicAuth, err := getBasicAuthInfo(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if hasBasicAuth {
		t.Errorf("Expected hasBasicAuth to be false")
	}
	if len(username) != 0 {
		t.Errorf("Unexpected username: %v", username)
	}
	if len(password) != 0 {
		t.Errorf("Unexpected password: %v", password)
	}
}
func TestGetBasicAuthInfoNotBase64Encoded(t *testing.T) {
	const (
		ExpectedError = "No valid base64 data in basic auth scheme found"
	)
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	req.Header.Add("Authorization", "Basic invalid:string")

	username, password, hasBasicAuth, err := getBasicAuthInfo(req)
	if err == nil {
		t.Errorf("Expected error: %v", ExpectedError)
	}
	if hasBasicAuth {
		t.Errorf("Expected hasBasicAuth to be false")
	}
	if err.Error() != ExpectedError {
		t.Errorf("Expected %v, got %v", ExpectedError, err)
	}
	if len(username) != 0 {
		t.Errorf("Unexpected username: %v", username)
	}
	if len(password) != 0 {
		t.Errorf("Unexpected password: %v", password)
	}
}
func TestGetBasicAuthInfoNotCredentials(t *testing.T) {
	const (
		ExpectedError = "Invalid Authorization header"
	)
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	req.Header.Add("Authorization", "Basic "+ValidBase64String)

	username, password, hasBasicAuth, err := getBasicAuthInfo(req)
	if err == nil {
		t.Errorf("Expected error: %v", ExpectedError)
	}
	if hasBasicAuth {
		t.Errorf("Expected hasBasicAuth to be false")
	}
	if err.Error() != ExpectedError {
		t.Errorf("Expected %v, got %v", ExpectedError, err)
	}
	if len(username) != 0 {
		t.Errorf("Unexpected username: %v", username)
	}
	if len(password) != 0 {
		t.Errorf("Unexpected password: %v", password)
	}
}
