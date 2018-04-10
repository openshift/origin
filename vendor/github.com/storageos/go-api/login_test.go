package storageos

import (
	"net/http"
	"testing"
)

func TestLoginOK(t *testing.T) {
	client := newTestClient(&FakeRoundTripper{message: `{"token":"superSecretToken"}`, status: http.StatusOK})
	client.SetAuth("foo", "bar")

	token, err := client.Login()
	if err != nil {
		t.Error(err)
	}

	if token != "superSecretToken" {
		t.Errorf("Token got garbled: (%v) != (superSecretToken)", token)
	}
}

func TestLoginBadCreds(t *testing.T) {
	client := newTestClient(&FakeRoundTripper{message: "", status: http.StatusUnauthorized})
	client.SetAuth("foo", "bar")

	token, err := client.Login()
	if err == nil {
		t.Error("Expected and error")
	}

	if err != ErrLoginFailed {
		t.Errorf("Expected login failed error got: %v", err)
	}

	if token != "" {
		t.Errorf("token (%v) incorrectly returned", token)
	}
}

func TestLoginEmptyToken(t *testing.T) {
	client := newTestClient(&FakeRoundTripper{message: `{"token":""}`, status: http.StatusOK})
	client.SetAuth("foo", "bar")

	_, err := client.Login()
	if err == nil {
		t.Error("Expected and error")
	}

	if err != ErrLoginFailed {
		t.Errorf("Expected login failed error got: %v", err)
	}
}
