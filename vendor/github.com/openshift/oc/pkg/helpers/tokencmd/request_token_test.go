package tokencmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/RangelReale/osincli"

	"k8s.io/apimachinery/pkg/util/diff"
	restclient "k8s.io/client-go/rest"

	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
)

type unloadableNegotiator struct {
	releaseCalls int
}

func (n *unloadableNegotiator) Load() error {
	return errors.New("Load failed")
}
func (n *unloadableNegotiator) InitSecContext(requestURL string, challengeToken []byte) (tokenToSend []byte, err error) {
	return nil, errors.New("InitSecContext failed")
}
func (n *unloadableNegotiator) IsComplete() bool {
	return false
}
func (n *unloadableNegotiator) Release() error {
	n.releaseCalls++
	return errors.New("Release failed")
}

type failingNegotiator struct {
	releaseCalls int
}

func (n *failingNegotiator) Load() error {
	return nil
}
func (n *failingNegotiator) InitSecContext(requestURL string, challengeToken []byte) (tokenToSend []byte, err error) {
	return nil, errors.New("InitSecContext failed")
}
func (n *failingNegotiator) IsComplete() bool {
	return false
}
func (n *failingNegotiator) Release() error {
	n.releaseCalls++
	return errors.New("Release failed")
}

type successfulNegotiator struct {
	rounds              int
	initSecContextCalls int
	loadCalls           int
	releaseCalls        int
}

func (n *successfulNegotiator) Load() error {
	n.loadCalls++
	return nil
}
func (n *successfulNegotiator) InitSecContext(requestURL string, challengeToken []byte) (tokenToSend []byte, err error) {
	n.initSecContextCalls++

	if n.initSecContextCalls > n.rounds {
		return nil, fmt.Errorf("InitSecContext: expected %d calls, saw %d", n.rounds, n.initSecContextCalls)
	}

	if n.initSecContextCalls == 1 {
		if len(challengeToken) > 0 {
			return nil, errors.New("expected empty token for first challenge")
		}
	} else {
		expectedChallengeToken := fmt.Sprintf("challenge%d", n.initSecContextCalls)
		if string(challengeToken) != expectedChallengeToken {
			return nil, fmt.Errorf("expected challenge token '%s', got '%s'", expectedChallengeToken, string(challengeToken))
		}
	}

	return []byte(fmt.Sprintf("response%d", n.initSecContextCalls)), nil
}
func (n *successfulNegotiator) IsComplete() bool {
	return n.initSecContextCalls == n.rounds
}
func (n *successfulNegotiator) Release() error {
	n.releaseCalls++
	return nil
}

func TestRequestToken(t *testing.T) {
	type req struct {
		authorization string
		method        string
		path          string
	}
	type resp struct {
		status          int
		location        string
		wwwAuthenticate []string
	}

	type requestResponse struct {
		expectedRequest req
		serverResponse  resp
	}

	var verifyReleased func(test string, handler ChallengeHandler)
	verifyReleased = func(test string, handler ChallengeHandler) {
		switch handler := handler.(type) {
		case *MultiHandler:
			for _, subhandler := range handler.allHandlers {
				verifyReleased(test, subhandler)
			}
		case *BasicChallengeHandler:
			// we don't care
		case *NegotiateChallengeHandler:
			switch negotiator := handler.negotiator.(type) {
			case *successfulNegotiator:
				if negotiator.releaseCalls != 1 {
					t.Errorf("%s: expected one call to Release(), saw %d", test, negotiator.releaseCalls)
				}
			case *failingNegotiator:
				if negotiator.releaseCalls != 1 {
					t.Errorf("%s: expected one call to Release(), saw %d", test, negotiator.releaseCalls)
				}
			case *unloadableNegotiator:
				if negotiator.releaseCalls != 1 {
					t.Errorf("%s: expected one call to Release(), saw %d", test, negotiator.releaseCalls)
				}
			default:
				t.Errorf("%s: unrecognized negotiator: %#v", test, handler)
			}
		default:
			t.Errorf("%s: unrecognized handler: %#v", test, handler)
		}
	}

	initialHead := req{"", http.MethodHead, "/"}
	initialHeadResp := resp{http.StatusInternalServerError, "", nil} // value of status is ignored

	initialRequest := req{}

	basicChallenge1 := resp{401, "", []string{"Basic realm=foo"}}
	basicRequest1 := req{"Basic bXl1c2VyOm15cGFzc3dvcmQ=", "", ""} // base64("myuser:mypassword")
	basicChallenge2 := resp{401, "", []string{"Basic realm=seriously...foo"}}

	negotiateChallenge1 := resp{401, "", []string{"Negotiate"}}
	negotiateRequest1 := req{"Negotiate cmVzcG9uc2Ux", "", ""}                   // base64("response1")
	negotiateChallenge2 := resp{401, "", []string{"Negotiate Y2hhbGxlbmdlMg=="}} // base64("challenge2")
	negotiateRequest2 := req{"Negotiate cmVzcG9uc2Uy", "", ""}                   // base64("response2")

	doubleChallenge := resp{401, "", []string{"Negotiate", "Basic realm=foo"}}

	successfulToken := "12345"
	successfulLocation := fmt.Sprintf("/#access_token=%s", successfulToken)
	success := resp{302, successfulLocation, nil}
	successWithNegotiate := resp{302, successfulLocation, []string{"Negotiate Y2hhbGxlbmdlMg=="}}

	testcases := map[string]struct {
		Handler       ChallengeHandler
		Requests      []requestResponse
		ExpectedToken string
		ExpectedError string
	}{
		// Defaulting basic handler
		"defaulted basic handler, no challenge, success": {
			Handler: &BasicChallengeHandler{Username: "myuser", Password: "mypassword"},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"defaulted basic handler, basic challenge, success": {
			Handler: &BasicChallengeHandler{Username: "myuser", Password: "mypassword"},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, basicChallenge1},
				{basicRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"defaulted basic handler, basic+negotiate challenge, success": {
			Handler: &BasicChallengeHandler{Username: "myuser", Password: "mypassword"},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, doubleChallenge},
				{basicRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"defaulted basic handler, basic challenge, failure": {
			Handler: &BasicChallengeHandler{Username: "myuser", Password: "mypassword"},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, basicChallenge1},
				{basicRequest1, basicChallenge2},
			},
			ExpectedError: "challenger chose not to retry the request",
		},
		"defaulted basic handler, negotiate challenge, failure": {
			Handler: &BasicChallengeHandler{Username: "myuser", Password: "mypassword"},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, negotiateChallenge1},
			},
			ExpectedError: "unhandled challenge",
		},
		"failing basic handler, basic challenge, failure": {
			Handler: &BasicChallengeHandler{},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, basicChallenge1},
			},
			ExpectedError: "challenger chose not to retry the request",
		},

		// Prompting basic handler
		"prompting basic handler, no challenge, success": {
			Handler: &BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"prompting basic handler, basic challenge, success": {
			Handler: &BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, basicChallenge1},
				{basicRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"prompting basic handler, basic+negotiate challenge, success": {
			Handler: &BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, doubleChallenge},
				{basicRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"prompting basic handler, basic challenge, failure": {
			Handler: &BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, basicChallenge1},
				{basicRequest1, basicChallenge2},
			},
			ExpectedError: "challenger chose not to retry the request",
		},
		"prompting basic handler, negotiate challenge, failure": {
			Handler: &BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, negotiateChallenge1},
			},
			ExpectedError: "unhandled challenge",
		},

		// negotiate handler
		"negotiate handler, no challenge, success": {
			Handler: &NegotiateChallengeHandler{negotiator: &successfulNegotiator{rounds: 1}},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate handler, negotiate challenge, success": {
			Handler: &NegotiateChallengeHandler{negotiator: &successfulNegotiator{rounds: 1}},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, negotiateChallenge1},
				{negotiateRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate handler, negotiate challenge, 2 rounds, success": {
			Handler: &NegotiateChallengeHandler{negotiator: &successfulNegotiator{rounds: 2}},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, negotiateChallenge1},
				{negotiateRequest1, negotiateChallenge2},
				{negotiateRequest2, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate handler, negotiate challenge, 2 rounds, success with mutual auth": {
			Handler: &NegotiateChallengeHandler{negotiator: &successfulNegotiator{rounds: 2}},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, negotiateChallenge1},
				{negotiateRequest1, successWithNegotiate},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate handler, negotiate challenge, 2 rounds expected, server success without client completion": {
			Handler: &NegotiateChallengeHandler{negotiator: &successfulNegotiator{rounds: 2}},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, negotiateChallenge1},
				{negotiateRequest1, success},
			},
			ExpectedError: "client requires final negotiate token, none provided",
		},

		// Unloadable negotiate handler
		"unloadable negotiate handler, no challenge, success": {
			Handler: &NegotiateChallengeHandler{negotiator: &unloadableNegotiator{}},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"unloadable negotiate handler, negotiate challenge, failure": {
			Handler: &NegotiateChallengeHandler{negotiator: &unloadableNegotiator{}},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, negotiateChallenge1},
			},
			ExpectedError: "unhandled challenge",
		},
		"unloadable negotiate handler, basic challenge, failure": {
			Handler: &NegotiateChallengeHandler{negotiator: &unloadableNegotiator{}},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, basicChallenge1},
			},
			ExpectedError: "unhandled challenge",
		},

		// Failing negotiate handler
		"failing negotiate handler, no challenge, success": {
			Handler: &NegotiateChallengeHandler{negotiator: &failingNegotiator{}},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"failing negotiate handler, negotiate challenge, failure": {
			Handler: &NegotiateChallengeHandler{negotiator: &failingNegotiator{}},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, negotiateChallenge1},
			},
			ExpectedError: "InitSecContext failed",
		},
		"failing negotiate handler, basic challenge, failure": {
			Handler: &NegotiateChallengeHandler{negotiator: &failingNegotiator{}},
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, basicChallenge1},
			},
			ExpectedError: "unhandled challenge",
		},

		// Negotiate+Basic fallback cases
		"failing negotiate+prompting basic handler, no challenge, success": {
			Handler: NewMultiHandler(
				&NegotiateChallengeHandler{negotiator: &failingNegotiator{}},
				&BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			),
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"failing negotiate+prompting basic handler, negotiate+basic challenge, success": {
			Handler: NewMultiHandler(
				&NegotiateChallengeHandler{negotiator: &failingNegotiator{}},
				&BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			),
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, doubleChallenge},
				{basicRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate+failing basic handler, negotiate+basic challenge, success": {
			Handler: NewMultiHandler(
				&NegotiateChallengeHandler{negotiator: &successfulNegotiator{rounds: 2}},
				&BasicChallengeHandler{},
			),
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, doubleChallenge},
				{negotiateRequest1, negotiateChallenge2},
				{negotiateRequest2, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate+basic handler, negotiate+basic challenge, prefers negotiation, success": {
			Handler: NewMultiHandler(
				&NegotiateChallengeHandler{negotiator: &successfulNegotiator{rounds: 2}},
				&BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			),
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, doubleChallenge},
				{negotiateRequest1, negotiateChallenge2},
				{negotiateRequest2, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate+basic handler, negotiate+basic challenge, prefers negotiation, sticks with selected handler on failure": {
			Handler: NewMultiHandler(
				&NegotiateChallengeHandler{negotiator: &successfulNegotiator{rounds: 2}},
				&BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			),
			Requests: []requestResponse{
				{initialHead, initialHeadResp},
				{initialRequest, doubleChallenge},
				{negotiateRequest1, negotiateChallenge2},
				{negotiateRequest2, doubleChallenge},
			},
			ExpectedError: "InitSecContext: expected 2 calls, saw 3",
		},
	}

	for k, tc := range testcases {
		i := 0
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					t.Errorf("test %s panicked: %v", k, err)
				}
			}()

			if i >= len(tc.Requests) {
				t.Errorf("%s: %d: more requests received than expected: %#v", k, i, req)
				return
			}
			rr := tc.Requests[i]
			i++

			method := rr.expectedRequest.method
			if len(method) == 0 {
				method = http.MethodGet
			}
			if req.Method != method {
				t.Errorf("%s: %d: Expected %s, got %s", k, i, method, req.Method)
				return
			}

			path := rr.expectedRequest.path
			if len(path) == 0 {
				path = "/oauth/authorize"
			}
			if req.URL.Path != path {
				t.Errorf("%s: %d: Expected %s, got %s", k, i, path, req.URL.Path)
				return
			}

			if e, a := rr.expectedRequest.authorization, req.Header.Get("Authorization"); e != a {
				t.Errorf("%s: %d: expected 'Authorization: %s', got 'Authorization: %s'", k, i, e, a)
				return
			}

			if len(rr.serverResponse.location) > 0 {
				w.Header().Add("Location", rr.serverResponse.location)
			}
			for _, v := range rr.serverResponse.wwwAuthenticate {
				w.Header().Add("WWW-Authenticate", v)
			}
			w.WriteHeader(rr.serverResponse.status)
		}))
		defer s.Close()

		opts := &RequestTokenOptions{
			ClientConfig: &restclient.Config{Host: s.URL},
			Handler:      tc.Handler,
			OsinConfig: &osincli.ClientConfig{
				ClientId:     openShiftCLIClientID,
				AuthorizeUrl: oauthdiscovery.OpenShiftOAuthAuthorizeURL(s.URL),
				TokenUrl:     oauthdiscovery.OpenShiftOAuthTokenURL(s.URL),
				RedirectUrl:  oauthdiscovery.OpenShiftOAuthTokenImplicitURL(s.URL),
			},
			Issuer:    s.URL,
			TokenFlow: true,
		}
		token, err := opts.RequestToken()
		if token != tc.ExpectedToken {
			t.Errorf("%s: expected token '%s', got '%s'", k, tc.ExpectedToken, token)
		}
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		if errStr != tc.ExpectedError {
			t.Errorf("%s: expected error '%s', got '%s'", k, tc.ExpectedError, errStr)
		}
		if i != len(tc.Requests) {
			t.Errorf("%s: expected %d requests, saw %d", k, len(tc.Requests), i)
		}
		verifyReleased(k, tc.Handler)
	}
}

func TestSetDefaultOsinConfig(t *testing.T) {
	noHostChange := func(host string) string { return host }
	for _, tc := range []struct {
		name        string
		metadata    *oauthdiscovery.OauthAuthorizationServerMetadata
		hostWrapper func(host string) (newHost string)
		tokenFlow   bool

		expectPKCE     bool
		expectedConfig *osincli.ClientConfig
	}{
		{
			name: "code with PKCE support from server",
			metadata: &oauthdiscovery.OauthAuthorizationServerMetadata{
				Issuer:                        "a",
				AuthorizationEndpoint:         "b",
				TokenEndpoint:                 "c",
				CodeChallengeMethodsSupported: []string{pkce_s256},
			},
			hostWrapper: noHostChange,
			tokenFlow:   false,

			expectPKCE: true,
			expectedConfig: &osincli.ClientConfig{
				ClientId:            openShiftCLIClientID,
				AuthorizeUrl:        "b",
				TokenUrl:            "c",
				RedirectUrl:         "a/oauth/token/implicit",
				CodeChallengeMethod: pkce_s256,
			},
		},
		{
			name: "code without PKCE support from server",
			metadata: &oauthdiscovery.OauthAuthorizationServerMetadata{
				Issuer:                        "a",
				AuthorizationEndpoint:         "b",
				TokenEndpoint:                 "c",
				CodeChallengeMethodsSupported: []string{"someotherstuff"},
			},
			hostWrapper: noHostChange,
			tokenFlow:   false,

			expectPKCE: false,
			expectedConfig: &osincli.ClientConfig{
				ClientId:     openShiftCLIClientID,
				AuthorizeUrl: "b",
				TokenUrl:     "c",
				RedirectUrl:  "a/oauth/token/implicit",
			},
		},
		{
			name: "token with PKCE support from server",
			metadata: &oauthdiscovery.OauthAuthorizationServerMetadata{
				Issuer:                        "a",
				AuthorizationEndpoint:         "b",
				TokenEndpoint:                 "c",
				CodeChallengeMethodsSupported: []string{pkce_s256},
			},
			hostWrapper: noHostChange,
			tokenFlow:   true,

			expectPKCE: false,
			expectedConfig: &osincli.ClientConfig{
				ClientId:     openShiftCLIClientID,
				AuthorizeUrl: "b",
				TokenUrl:     "c",
				RedirectUrl:  "a/oauth/token/implicit",
			},
		},
		{
			name: "code with PKCE support from server, but wrong case",
			metadata: &oauthdiscovery.OauthAuthorizationServerMetadata{
				Issuer:                        "a",
				AuthorizationEndpoint:         "b",
				TokenEndpoint:                 "c",
				CodeChallengeMethodsSupported: []string{"s256"}, // we are case sensitive so this is not valid
			},
			hostWrapper: noHostChange,
			tokenFlow:   false,

			expectPKCE: false,
			expectedConfig: &osincli.ClientConfig{
				ClientId:     openShiftCLIClientID,
				AuthorizeUrl: "b",
				TokenUrl:     "c",
				RedirectUrl:  "a/oauth/token/implicit",
			},
		},
		{
			name: "token without PKCE support from server",
			metadata: &oauthdiscovery.OauthAuthorizationServerMetadata{
				Issuer:                        "a",
				AuthorizationEndpoint:         "b",
				TokenEndpoint:                 "c",
				CodeChallengeMethodsSupported: []string{"random"},
			},
			hostWrapper: noHostChange,
			tokenFlow:   true,

			expectPKCE: false,
			expectedConfig: &osincli.ClientConfig{
				ClientId:     openShiftCLIClientID,
				AuthorizeUrl: "b",
				TokenUrl:     "c",
				RedirectUrl:  "a/oauth/token/implicit",
			},
		},
		{
			name: "host with extra slashes",
			metadata: &oauthdiscovery.OauthAuthorizationServerMetadata{
				Issuer:                        "a",
				AuthorizationEndpoint:         "b",
				TokenEndpoint:                 "c",
				CodeChallengeMethodsSupported: []string{pkce_s256},
			},
			hostWrapper: func(host string) string { return host + "/////" },
			tokenFlow:   false,

			expectPKCE: true,
			expectedConfig: &osincli.ClientConfig{
				ClientId:            openShiftCLIClientID,
				AuthorizeUrl:        "b",
				TokenUrl:            "c",
				RedirectUrl:         "a/oauth/token/implicit",
				CodeChallengeMethod: pkce_s256,
			},
		},
		{
			name: "issuer with extra slashes",
			metadata: &oauthdiscovery.OauthAuthorizationServerMetadata{
				Issuer:                        "a/////",
				AuthorizationEndpoint:         "b",
				TokenEndpoint:                 "c",
				CodeChallengeMethodsSupported: []string{pkce_s256},
			},
			hostWrapper: noHostChange,
			tokenFlow:   false,

			expectPKCE: true,
			expectedConfig: &osincli.ClientConfig{
				ClientId:            openShiftCLIClientID,
				AuthorizeUrl:        "b",
				TokenUrl:            "c",
				RedirectUrl:         "a/oauth/token/implicit",
				CodeChallengeMethod: pkce_s256,
			},
		},
		{
			name: "code with PKCE support from server, more complex JSON",
			metadata: &oauthdiscovery.OauthAuthorizationServerMetadata{
				Issuer:                        "arandomissuerthatisfun123!!!///",
				AuthorizationEndpoint:         "44authzisanawesomeendpoint",
				TokenEndpoint:                 "&&buttokenendpointisprettygoodtoo",
				CodeChallengeMethodsSupported: []string{pkce_s256},
			},
			hostWrapper: noHostChange,
			tokenFlow:   false,

			expectPKCE: true,
			expectedConfig: &osincli.ClientConfig{
				ClientId:            openShiftCLIClientID,
				AuthorizeUrl:        "44authzisanawesomeendpoint",
				TokenUrl:            "&&buttokenendpointisprettygoodtoo",
				RedirectUrl:         "arandomissuerthatisfun123!!!/oauth/token/implicit",
				CodeChallengeMethod: pkce_s256,
			},
		},
	} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method != "GET" {
				t.Errorf("%s: Expected GET, got %s", tc.name, req.Method)
				return
			}
			if req.URL.Path != oauthMetadataEndpoint {
				t.Errorf("%s: Expected metadata endpoint, got %s", tc.name, req.URL.Path)
				return
			}
			data, err := json.Marshal(tc.metadata)
			if err != nil {
				t.Errorf("%s: unexpected json error: %v", tc.name, err)
				return
			}
			w.Write(data)
		}))
		defer s.Close()

		opts := &RequestTokenOptions{
			ClientConfig: &restclient.Config{Host: tc.hostWrapper(s.URL)},
			TokenFlow:    tc.tokenFlow,
		}
		if err := opts.SetDefaultOsinConfig(); err != nil {
			t.Errorf("%s: unexpected SetDefaultOsinConfig error: %v", tc.name, err)
			continue
		}

		// check PKCE data
		if tc.expectPKCE {
			if len(opts.OsinConfig.CodeChallenge) == 0 || len(opts.OsinConfig.CodeChallengeMethod) == 0 || len(opts.OsinConfig.CodeVerifier) == 0 {
				t.Errorf("%s: did not set PKCE", tc.name)
				continue
			}
		} else {
			if len(opts.OsinConfig.CodeChallenge) != 0 || len(opts.OsinConfig.CodeChallengeMethod) != 0 || len(opts.OsinConfig.CodeVerifier) != 0 {
				t.Errorf("%s: incorrectly set PKCE", tc.name)
				continue
			}
		}

		// blindly unset random PKCE data since we already checked for it
		opts.OsinConfig.CodeChallenge = ""
		opts.OsinConfig.CodeVerifier = ""

		// compare the configs to see if they match
		if !reflect.DeepEqual(*tc.expectedConfig, *opts.OsinConfig) {
			t.Errorf("%s: expected osin config does not match, %s", tc.name, diff.ObjectDiff(*tc.expectedConfig, *opts.OsinConfig))
		}
	}
}
