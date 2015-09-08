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
		t.Errorf("Unexpected error: %v", err)
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
		t.Errorf("Unexpected error: %v", err)
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
		t.Errorf("Unexpected error: %v", err)
	}
	if !handled {
		t.Error("Expected handling.")
	}
	if !(reflect.DeepEqual(map[string][]string(responseRecorder.HeaderMap), expectedHeader1) || reflect.DeepEqual(map[string][]string(responseRecorder.HeaderMap), expectedHeader2)) {
		t.Errorf("Expected %#v or %#v, got %#v.", expectedHeader1, expectedHeader2, responseRecorder.HeaderMap)
	}
	if responseRecorder.Code != 401 {
		t.Errorf("Expected 401, got %d", responseRecorder.Code)
	}
}

func TestWithChallengeAndRedirect(t *testing.T) {
	expectedError := "Location"
	workingChallengeHandler1 := &mockChallenger{headerName: "Location", headerValue: "https://example.com"}
	workingChallengeHandler2 := &mockChallenger{headerName: "WWW-Authenticate", headerValue: "Basic"}

	authHandler := NewUnionAuthenticationHandler(
		map[string]AuthenticationChallenger{
			"first":  workingChallengeHandler1,
			"second": workingChallengeHandler2,
		}, nil, nil)
	client := &testClient{&oauthapi.OAuthClient{RespondWithChallenges: true}}
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	responseRecorder := httptest.NewRecorder()
	handled, err := authHandler.AuthenticationNeeded(client, responseRecorder, req)

	if err == nil {
		t.Errorf("Expected error, got none")
	} else if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing %q, got %v", expectedError, err)
	}
	if handled {
		t.Error("Unexpected handling.")
	}
}

func TestWithRedirect(t *testing.T) {
	workingChallengeHandler1 := &mockChallenger{headerName: "Location", headerValue: "https://example.com"}

	// order of the array is not guaranteed
	expectedHeader1 := map[string][]string{"Location": {"https://example.com"}}

	authHandler := NewUnionAuthenticationHandler(
		map[string]AuthenticationChallenger{
			"first": workingChallengeHandler1,
		},
		nil, nil)
	client := &testClient{&oauthapi.OAuthClient{RespondWithChallenges: true}}
	req, _ := http.NewRequest("GET", "http://example.org", nil)
	responseRecorder := httptest.NewRecorder()
	handled, err := authHandler.AuthenticationNeeded(client, responseRecorder, req)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !handled {
		t.Error("Expected handling.")
	}
	if !reflect.DeepEqual(map[string][]string(responseRecorder.HeaderMap), expectedHeader1) {
		t.Errorf("Expected %#v, got %#v.", expectedHeader1, responseRecorder.HeaderMap)
	}
	if responseRecorder.Code != 302 {
		t.Errorf("Expected 302, got %d", responseRecorder.Code)
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

func TestWarningRegex(t *testing.T) {
	testcases := map[string]struct {
		Header string
		Match  bool
		Parts  []string
	}{
		// Empty
		"empty": {},

		// Invalid code segment
		"code 2 numbers":   {Header: `19 Origin "Message goes here"`},
		"code 4 numbers":   {Header: `1999 Origin "Message goes here"`},
		"code non-numbers": {Header: `ABC Origin "Message goes here"`},

		// Invalid agent segment
		"agent missing": {Header: `199  "Message goes here"`},
		"agent spaces":  {Header: `199 Open Shift "Message goes here"`},

		// Invalid text segment
		"text missing":       {Header: `199 Origin`},
		"text unquoted":      {Header: `199 Origin Message`},
		"text single quotes": {Header: `199 Origin 'Message'`},
		"text bad quotes":    {Header: `199 Origin "Mes"sage"`},
		"text bad escape":    {Header: `199 Origin "Mes\\"sage"`},

		// Invalid date segment
		"date unquoted":      {Header: `199 Origin "Message" Date`},
		"date single quoted": {Header: `199 Origin "Message" 'Date'`},
		"date empty":         {Header: `199 Origin "Message" ""`},

		// Valid segments
		"valid no date": {
			Header: `199 Origin "Message goes here"`,
			Match:  true,
			Parts:  []string{"199", "Origin", "Message goes here", ""},
		},

		"valid with date": {
			Header: `199 Origin "Message goes here" "date"`,
			Match:  true,
			Parts:  []string{"199", "Origin", "Message goes here", "date"},
		},

		"valid with escaped quote": {
			Header: `199 Origin "Message \" goes here" "date"`,
			Match:  true,
			Parts:  []string{"199", "Origin", `Message \" goes here`, "date"},
		},

		"valid with escaped quote and slash": {
			Header: `199 Origin "Message \\\" goes here" "date"`,
			Match:  true,
			Parts:  []string{"199", "Origin", `Message \\\" goes here`, "date"},
		},
	}

	for k, tc := range testcases {
		parts := warningRegex.FindStringSubmatch(tc.Header)
		match := len(parts) > 0
		if match != tc.Match {
			t.Errorf("%s: Expected match %v, got %v", k, tc.Match, match)
			continue
		}
		if !match {
			continue
		}
		if !reflect.DeepEqual(parts[1:], tc.Parts) {
			t.Errorf("%s: Expected\n\t%#v\n\tgot\n\t%#v", k, tc.Parts, parts[1:])
		}
	}
}
