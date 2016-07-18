package tokencmd

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/kubernetes/pkg/client/restclient"
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
			switch negotiator := handler.negotiater.(type) {
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

	initialRequest := req{}

	basicChallenge1 := resp{401, "", []string{"Basic realm=foo"}}
	basicRequest1 := req{"Basic bXl1c2VyOm15cGFzc3dvcmQ="} // base64("myuser:mypassword")
	basicChallenge2 := resp{401, "", []string{"Basic realm=seriously...foo"}}

	negotiateChallenge1 := resp{401, "", []string{"Negotiate"}}
	negotiateRequest1 := req{"Negotiate cmVzcG9uc2Ux"}                           // base64("response1")
	negotiateChallenge2 := resp{401, "", []string{"Negotiate Y2hhbGxlbmdlMg=="}} // base64("challenge2")
	negotiateRequest2 := req{"Negotiate cmVzcG9uc2Uy"}                           // base64("response2")

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
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"defaulted basic handler, basic challenge, success": {
			Handler: &BasicChallengeHandler{Username: "myuser", Password: "mypassword"},
			Requests: []requestResponse{
				{initialRequest, basicChallenge1},
				{basicRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"defaulted basic handler, basic+negotiate challenge, success": {
			Handler: &BasicChallengeHandler{Username: "myuser", Password: "mypassword"},
			Requests: []requestResponse{
				{initialRequest, doubleChallenge},
				{basicRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"defaulted basic handler, basic challenge, failure": {
			Handler: &BasicChallengeHandler{Username: "myuser", Password: "mypassword"},
			Requests: []requestResponse{
				{initialRequest, basicChallenge1},
				{basicRequest1, basicChallenge2},
			},
			ExpectedError: "challenger chose not to retry the request",
		},
		"defaulted basic handler, negotiate challenge, failure": {
			Handler: &BasicChallengeHandler{Username: "myuser", Password: "mypassword"},
			Requests: []requestResponse{
				{initialRequest, negotiateChallenge1},
			},
			ExpectedError: "unhandled challenge",
		},
		"failing basic handler, basic challenge, failure": {
			Handler: &BasicChallengeHandler{},
			Requests: []requestResponse{
				{initialRequest, basicChallenge1},
			},
			ExpectedError: "challenger chose not to retry the request",
		},

		// Prompting basic handler
		"prompting basic handler, no challenge, success": {
			Handler: &BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			Requests: []requestResponse{
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"prompting basic handler, basic challenge, success": {
			Handler: &BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			Requests: []requestResponse{
				{initialRequest, basicChallenge1},
				{basicRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"prompting basic handler, basic+negotiate challenge, success": {
			Handler: &BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			Requests: []requestResponse{
				{initialRequest, doubleChallenge},
				{basicRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"prompting basic handler, basic challenge, failure": {
			Handler: &BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			Requests: []requestResponse{
				{initialRequest, basicChallenge1},
				{basicRequest1, basicChallenge2},
			},
			ExpectedError: "challenger chose not to retry the request",
		},
		"prompting basic handler, negotiate challenge, failure": {
			Handler: &BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			Requests: []requestResponse{
				{initialRequest, negotiateChallenge1},
			},
			ExpectedError: "unhandled challenge",
		},

		// negotiate handler
		"negotiate handler, no challenge, success": {
			Handler: &NegotiateChallengeHandler{negotiater: &successfulNegotiator{rounds: 1}},
			Requests: []requestResponse{
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate handler, negotiate challenge, success": {
			Handler: &NegotiateChallengeHandler{negotiater: &successfulNegotiator{rounds: 1}},
			Requests: []requestResponse{
				{initialRequest, negotiateChallenge1},
				{negotiateRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate handler, negotiate challenge, 2 rounds, success": {
			Handler: &NegotiateChallengeHandler{negotiater: &successfulNegotiator{rounds: 2}},
			Requests: []requestResponse{
				{initialRequest, negotiateChallenge1},
				{negotiateRequest1, negotiateChallenge2},
				{negotiateRequest2, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate handler, negotiate challenge, 2 rounds, success with mutual auth": {
			Handler: &NegotiateChallengeHandler{negotiater: &successfulNegotiator{rounds: 2}},
			Requests: []requestResponse{
				{initialRequest, negotiateChallenge1},
				{negotiateRequest1, successWithNegotiate},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate handler, negotiate challenge, 2 rounds expected, server success without client completion": {
			Handler: &NegotiateChallengeHandler{negotiater: &successfulNegotiator{rounds: 2}},
			Requests: []requestResponse{
				{initialRequest, negotiateChallenge1},
				{negotiateRequest1, success},
			},
			ExpectedError: "client requires final negotiate token, none provided",
		},

		// Unloadable negotiate handler
		"unloadable negotiate handler, no challenge, success": {
			Handler: &NegotiateChallengeHandler{negotiater: &unloadableNegotiator{}},
			Requests: []requestResponse{
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"unloadable negotiate handler, negotiate challenge, failure": {
			Handler: &NegotiateChallengeHandler{negotiater: &unloadableNegotiator{}},
			Requests: []requestResponse{
				{initialRequest, negotiateChallenge1},
			},
			ExpectedError: "unhandled challenge",
		},
		"unloadable negotiate handler, basic challenge, failure": {
			Handler: &NegotiateChallengeHandler{negotiater: &unloadableNegotiator{}},
			Requests: []requestResponse{
				{initialRequest, basicChallenge1},
			},
			ExpectedError: "unhandled challenge",
		},

		// Failing negotiate handler
		"failing negotiate handler, no challenge, success": {
			Handler: &NegotiateChallengeHandler{negotiater: &failingNegotiator{}},
			Requests: []requestResponse{
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"failing negotiate handler, negotiate challenge, failure": {
			Handler: &NegotiateChallengeHandler{negotiater: &failingNegotiator{}},
			Requests: []requestResponse{
				{initialRequest, negotiateChallenge1},
			},
			ExpectedError: "InitSecContext failed",
		},
		"failing negotiate handler, basic challenge, failure": {
			Handler: &NegotiateChallengeHandler{negotiater: &failingNegotiator{}},
			Requests: []requestResponse{
				{initialRequest, basicChallenge1},
			},
			ExpectedError: "unhandled challenge",
		},

		// Negotiate+Basic fallback cases
		"failing negotiate+prompting basic handler, no challenge, success": {
			Handler: NewMultiHandler(
				&NegotiateChallengeHandler{negotiater: &failingNegotiator{}},
				&BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			),
			Requests: []requestResponse{
				{initialRequest, success},
			},
			ExpectedToken: successfulToken,
		},
		"failing negotiate+prompting basic handler, negotiate+basic challenge, success": {
			Handler: NewMultiHandler(
				&NegotiateChallengeHandler{negotiater: &failingNegotiator{}},
				&BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			),
			Requests: []requestResponse{
				{initialRequest, doubleChallenge},
				{basicRequest1, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate+failing basic handler, negotiate+basic challenge, success": {
			Handler: NewMultiHandler(
				&NegotiateChallengeHandler{negotiater: &successfulNegotiator{rounds: 2}},
				&BasicChallengeHandler{},
			),
			Requests: []requestResponse{
				{initialRequest, doubleChallenge},
				{negotiateRequest1, negotiateChallenge2},
				{negotiateRequest2, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate+basic handler, negotiate+basic challenge, prefers negotiation, success": {
			Handler: NewMultiHandler(
				&NegotiateChallengeHandler{negotiater: &successfulNegotiator{rounds: 2}},
				&BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			),
			Requests: []requestResponse{
				{initialRequest, doubleChallenge},
				{negotiateRequest1, negotiateChallenge2},
				{negotiateRequest2, success},
			},
			ExpectedToken: successfulToken,
		},
		"negotiate+basic handler, negotiate+basic challenge, prefers negotiation, sticks with selected handler on failure": {
			Handler: NewMultiHandler(
				&NegotiateChallengeHandler{negotiater: &successfulNegotiator{rounds: 2}},
				&BasicChallengeHandler{Reader: bytes.NewBufferString("myuser\nmypassword\n")},
			),
			Requests: []requestResponse{
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
			if i > len(tc.Requests) {
				t.Errorf("%s: %d: more requests received than expected: %#v", k, i, req)
				return
			}
			rr := tc.Requests[i]
			i++
			if req.Method != "GET" {
				t.Errorf("%s: %d: Expected GET, got %s", k, i, req.Method)
				return
			}
			if req.URL.Path != "/oauth/authorize" {
				t.Errorf("%s: %d: Expected /oauth/authorize, got %s", k, i, req.URL.Path)
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
