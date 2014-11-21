package requesthandlers

import (
	"net/http"
	"testing"

	authapi "github.com/openshift/origin/pkg/auth/api"
)

const (
	USERNAME            = "frightened_donut"
	PASSWORD            = "don't eat me!"
	VALID_BASE64_STRING = "VGhpc0lzVmFsaWQK" // base64 -- ThisIsValid ctrl+d
)

type mockPasswordAuthenticator struct {
	returnUser      authapi.UserInfo
	isAuthenticated bool
	err             error
	passedUser      string
	passedPassword  string
}

func (mock *mockPasswordAuthenticator) AuthenticatePassword(username, password string) (authapi.UserInfo, bool, error) {
	mock.passedUser = username
	mock.passedPassword = password

	return mock.returnUser, mock.isAuthenticated, mock.err
}

func TestAuthenticateRequestValid(t *testing.T) {
	passwordAuthenticator := &mockPasswordAuthenticator{}
	authRequestHandler := NewBasicAuthAuthentication(passwordAuthenticator)
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	req.SetBasicAuth(USERNAME, PASSWORD)

	_, _, _ = authRequestHandler.AuthenticateRequest(req)
	if passwordAuthenticator.passedUser != USERNAME {
		t.Errorf("Expected %v, got %v", USERNAME, passwordAuthenticator.passedUser)
	}
	if passwordAuthenticator.passedPassword != PASSWORD {
		t.Errorf("Expected %v, got %v", PASSWORD, passwordAuthenticator.passedPassword)
	}
}

func TestAuthenticateRequestInvalid(t *testing.T) {
	const (
		EXPECTED_ERROR = "No valid base64 data in basic auth scheme found"
	)
	passwordAuthenticator := &mockPasswordAuthenticator{isAuthenticated: true}
	authRequestHandler := NewBasicAuthAuthentication(passwordAuthenticator)
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	req.Header.Add("Authorization", "Basic invalid:string")

	userInfo, authenticated, err := authRequestHandler.AuthenticateRequest(req)
	if err == nil {
		t.Errorf("Expected error: %v", EXPECTED_ERROR)
	}
	if err.Error() != EXPECTED_ERROR {
		t.Errorf("Expected %v, got %v", EXPECTED_ERROR, err)
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
	req.SetBasicAuth(USERNAME, PASSWORD)

	username, password, err := getBasicAuthInfo(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if username != USERNAME {
		t.Errorf("Expected %v, got %v", USERNAME, username)
	}
	if password != PASSWORD {
		t.Errorf("Expected %v, got %v", PASSWORD, password)
	}
}

func TestGetBasicAuthInfoNoHeader(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.org", nil)

	username, password, err := getBasicAuthInfo(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
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

	username, password, err := getBasicAuthInfo(req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
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
		EXPECTED_ERROR = "No valid base64 data in basic auth scheme found"
	)
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	req.Header.Add("Authorization", "Basic invalid:string")

	username, password, err := getBasicAuthInfo(req)
	if err == nil {
		t.Errorf("Expected error: %v", EXPECTED_ERROR)
	}
	if err.Error() != EXPECTED_ERROR {
		t.Errorf("Expected %v, got %v", EXPECTED_ERROR, err)
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
		EXPECTED_ERROR = "Invalid Authorization header"
	)
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	req.Header.Add("Authorization", "Basic "+VALID_BASE64_STRING)

	username, password, err := getBasicAuthInfo(req)
	if err == nil {
		t.Errorf("Expected error: %v", EXPECTED_ERROR)
	}
	if err.Error() != EXPECTED_ERROR {
		t.Errorf("Expected %v, got %v", EXPECTED_ERROR, err)
	}
	if len(username) != 0 {
		t.Errorf("Unexpected username: %v", username)
	}
	if len(password) != 0 {
		t.Errorf("Unexpected password: %v", password)
	}
}
