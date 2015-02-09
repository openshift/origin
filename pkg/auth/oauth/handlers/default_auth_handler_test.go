package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	oauthapi "github.com/openshift/origin/pkg/oauth/api"
)

type testClient struct {
	client *oauthapi.OAuthClient
}

func (w *testClient) GetId() string {
	return w.client.Name
}

func (w *testClient) GetSecret() string {
	return w.client.Secret
}

func (w *testClient) GetRedirectUri() string {
	if len(w.client.RedirectURIs) == 0 {
		return ""
	}
	return strings.Join(w.client.RedirectURIs, ",")
}

func (w *testClient) GetUserData() interface{} {
	return w.client
}

type mockChallenger struct {
	headerName  string
	headerValue string
	err         error
}

func (h *mockChallenger) AuthenticationChallenge(req *http.Request) (http.Header, error) {
	headers := http.Header{}
	if len(h.headerName) > 0 {
		headers.Add(h.headerName, h.headerValue)
	}

	return headers, h.err
}

func TestNoHandlersRedirect(t *testing.T) {
	authHandler := NewUnionAuthenticationHandler(nil, nil, nil)
	client := &testClient{&oauthapi.OAuthClient{}}
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	responseRecorder := httptest.NewRecorder()
	handled, err := authHandler.AuthenticationNeeded(client, responseRecorder, req)

	if err != nil {
		t.Errorf("Unexepcted error: %v", err)
	}

	if handled {
		t.Error("Unexpectedly handled.")
	}
}

func TestNoHandlersChallenge(t *testing.T) {
	authHandler := NewUnionAuthenticationHandler(nil, nil, nil)
	client := &testClient{&oauthapi.OAuthClient{RespondWithChallenges: true}}
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	responseRecorder := httptest.NewRecorder()
	handled, err := authHandler.AuthenticationNeeded(client, responseRecorder, req)

	if err != nil {
		t.Errorf("Unexepcted error: %v", err)
	}

	if handled {
		t.Error("Unexpectedly handled.")
	}
}

func TestWithBadClient(t *testing.T) {
	authHandler := NewUnionAuthenticationHandler(nil, nil, nil)
	client := &badTestClient{&oauthapi.OAuthClient{}}
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	responseRecorder := httptest.NewRecorder()
	handled, err := authHandler.AuthenticationNeeded(client, responseRecorder, req)

	expectedError := "apiClient data was not an oauthapi.OAuthClient"
	if err == nil {
		t.Errorf("Expected error: %v", expectedError)
	}
	if err.Error() != expectedError {
		t.Errorf("Expected %v, got %v", expectedError, err)
	}

	if handled {
		t.Error("Unexpectedly handled.")
	}
}

func TestWithOnlyChallengeErrors(t *testing.T) {
	expectedError1 := "alfa"
	expectedError2 := "bravo"
	failingChallengeHandler1 := &mockChallenger{err: errors.New(expectedError1)}
	failingChallengeHandler2 := &mockChallenger{err: errors.New(expectedError2)}
	authHandler := NewUnionAuthenticationHandler(
		map[string]AuthenticationChallenger{"first": failingChallengeHandler1, "second": failingChallengeHandler2},
		nil, nil)
	client := &testClient{&oauthapi.OAuthClient{RespondWithChallenges: true}}
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	responseRecorder := httptest.NewRecorder()
	handled, err := authHandler.AuthenticationNeeded(client, responseRecorder, req)

	if err == nil {
		t.Errorf("Expected error: %v and %v", expectedError1, expectedError2)
	}
	if !strings.Contains(err.Error(), expectedError1) {
		t.Errorf("Expected %v, got %v", expectedError1, err)
	}
	if !strings.Contains(err.Error(), expectedError2) {
		t.Errorf("Expected %v, got %v", expectedError2, err)
	}

	if handled {
		t.Error("Unexpectedly handled.")
	}
}

func TestWithChallengeErrorsAndMergedSuccess(t *testing.T) {
	expectedError := "failure"
	failingChallengeHandler := &mockChallenger{err: errors.New(expectedError)}
	workingChallengeHandler1 := &mockChallenger{headerName: "Charlie", headerValue: "delta"}
	workingChallengeHandler2 := &mockChallenger{headerName: "Echo", headerValue: "foxtrot"}
	workingChallengeHandler3 := &mockChallenger{headerName: "Charlie", headerValue: "golf"}

	// order of the array is not guaranteed
	expectedHeader1 := map[string][]string{"Charlie": {"delta", "golf"}, "Echo": {"foxtrot"}}
	expectedHeader2 := map[string][]string{"Charlie": {"golf", "delta"}, "Echo": {"foxtrot"}}

	authHandler := NewUnionAuthenticationHandler(
		map[string]AuthenticationChallenger{
			"first":  failingChallengeHandler,
			"second": workingChallengeHandler1,
			"third":  workingChallengeHandler2,
			"fourth": workingChallengeHandler3},
		nil, nil)
	client := &testClient{&oauthapi.OAuthClient{RespondWithChallenges: true}}
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	responseRecorder := httptest.NewRecorder()
	handled, err := authHandler.AuthenticationNeeded(client, responseRecorder, req)

	if err != nil {
		t.Errorf("Unexepcted error: %v", err)
	}
	if !handled {
		t.Error("Expected handling.")
	}
	if !(reflect.DeepEqual(map[string][]string(responseRecorder.HeaderMap), expectedHeader1) || reflect.DeepEqual(map[string][]string(responseRecorder.HeaderMap), expectedHeader2)) {
		t.Errorf("Expected %#v or %#v, got %#v.", expectedHeader1, expectedHeader2, responseRecorder.HeaderMap)
	}
}

type badTestClient struct {
	client *oauthapi.OAuthClient
}

func (w *badTestClient) GetId() string {
	return w.client.Name
}

func (w *badTestClient) GetSecret() string {
	return w.client.Secret
}

func (w *badTestClient) GetRedirectUri() string {
	if len(w.client.RedirectURIs) == 0 {
		return ""
	}
	return strings.Join(w.client.RedirectURIs, ",")
}

func (w *badTestClient) GetUserData() interface{} {
	return "w.client"
}
