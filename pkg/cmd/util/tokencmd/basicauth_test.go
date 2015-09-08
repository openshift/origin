package tokencmd

import (
	"bytes"
	"net/http"
	"reflect"
	"testing"
)

var (
	AUTHORIZATION    = http.CanonicalHeaderKey("Authorization")
	WWW_AUTHENTICATE = http.CanonicalHeaderKey("WWW-Authenticate")
)

type Challenge struct {
	Headers http.Header

	ExpectedCanHandle bool
	ExpectedHeaders   http.Header
	ExpectedHandled   bool
	ExpectedErr       error
	ExpectedPrompt    string
}

func TestHandleChallenge(t *testing.T) {

	basicChallenge := http.Header{WWW_AUTHENTICATE: []string{`Basic realm="myrealm"`}}

	testCases := map[string]struct {
		Handler    *BasicChallengeHandler
		Challenges []Challenge
	}{
		"non-interactive with no defaults": {
			Handler: &BasicChallengeHandler{
				Host:     "myhost",
				Reader:   nil,
				Username: "",
				Password: "",
			},
			Challenges: []Challenge{
				{
					Headers:           basicChallenge,
					ExpectedCanHandle: true,
					ExpectedHeaders:   nil,
					ExpectedHandled:   false,
					ExpectedErr:       nil,
					ExpectedPrompt:    "",
				},
			},
		},

		"non-interactive challenge with defaults": {
			Handler: &BasicChallengeHandler{
				Host:     "myhost",
				Reader:   nil,
				Username: "myuser",
				Password: "mypassword",
			},
			Challenges: []Challenge{
				{
					Headers:           basicChallenge,
					ExpectedCanHandle: true,
					ExpectedHeaders:   http.Header{AUTHORIZATION: []string{getBasicHeader("myuser", "mypassword")}},
					ExpectedHandled:   true,
					ExpectedErr:       nil,
					ExpectedPrompt:    "",
				},
				{
					Headers:           basicChallenge,
					ExpectedCanHandle: true,
					ExpectedHeaders:   nil,
					ExpectedHandled:   false,
					ExpectedErr:       nil,
					ExpectedPrompt:    "",
				},
			},
		},

		"interactive challenge with default user": {
			Handler: &BasicChallengeHandler{
				Host:     "myhost",
				Reader:   bytes.NewBufferString("mypassword\n"),
				Username: "myuser",
				Password: "",
			},
			Challenges: []Challenge{
				{
					Headers:           basicChallenge,
					ExpectedCanHandle: true,
					ExpectedHeaders:   http.Header{AUTHORIZATION: []string{getBasicHeader("myuser", "mypassword")}},
					ExpectedHandled:   true,
					ExpectedErr:       nil,
					ExpectedPrompt: `Authentication required for myhost (myrealm)
Username: myuser
Password: `,
				},
			},
		},

		"interactive challenge": {
			Handler: &BasicChallengeHandler{
				Host:     "myhost",
				Reader:   bytes.NewBufferString("myuser\nmypassword\n"),
				Username: "",
				Password: "",
			},
			Challenges: []Challenge{
				{
					Headers:           basicChallenge,
					ExpectedCanHandle: true,
					ExpectedHeaders:   http.Header{AUTHORIZATION: []string{getBasicHeader("myuser", "mypassword")}},
					ExpectedHandled:   true,
					ExpectedErr:       nil,
					ExpectedPrompt: `Authentication required for myhost (myrealm)
Username: Password: `,
				},
				{
					Headers:           basicChallenge,
					ExpectedCanHandle: true,
					ExpectedHeaders:   nil,
					ExpectedHandled:   false,
					ExpectedErr:       nil,
					ExpectedPrompt:    ``,
				},
			},
		},
	}

	for k, tc := range testCases {
		for i, challenge := range tc.Challenges {
			out := &bytes.Buffer{}
			tc.Handler.Writer = out

			canHandle := tc.Handler.CanHandle(challenge.Headers)
			if canHandle != challenge.ExpectedCanHandle {
				t.Errorf("%s: %d: Expected CanHandle=%v, got %v", k, i, challenge.ExpectedCanHandle, canHandle)
			}

			if canHandle {
				headers, handled, err := tc.Handler.HandleChallenge(challenge.Headers)
				if !reflect.DeepEqual(headers, challenge.ExpectedHeaders) {
					t.Errorf("%s: %d: Expected headers\n\t%#v\ngot\n\t%#v", k, i, challenge.ExpectedHeaders, headers)
				}
				if handled != challenge.ExpectedHandled {
					t.Errorf("%s: %d: Expected handled=%v, got %v", k, i, challenge.ExpectedHandled, handled)
				}
				if err != challenge.ExpectedErr {
					t.Errorf("%s: %d: Expected err=%v, got %v", k, i, challenge.ExpectedErr, err)
				}
				if out.String() != challenge.ExpectedPrompt {
					t.Errorf("%s: %d: Expected prompt %q, got %q", k, i, challenge.ExpectedPrompt, out.String())
				}
			}
		}
	}
}

func TestBasicRealm(t *testing.T) {

	testCases := map[string]struct {
		Headers       http.Header
		ExpectedBasic bool
		ExpectedRealm string
	}{
		"empty": {
			Headers:       http.Header{},
			ExpectedBasic: false,
			ExpectedRealm: ``,
		},

		"non-challenge": {
			Headers: http.Header{
				"test": []string{`value`},
			},
			ExpectedBasic: false,
			ExpectedRealm: ``,
		},

		"non-basic": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{
					`basicrealm="myrealm"`,
					`digest basic="realm"`,
				},
			},
			ExpectedBasic: false,
			ExpectedRealm: ``,
		},

		"basic multiple www-authenticate headers": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{
					`digest realm="digestrealm"`,
					`basic realm="Foo"`,
					`foo bar="baz"`,
				},
			},
			ExpectedBasic: true,
			ExpectedRealm: `Foo`,
		},

		"basic no realm": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{`basic`},
			},
			ExpectedBasic: true,
			ExpectedRealm: ``,
		},

		"basic other param": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{`basic otherparam="othervalue"`},
			},
			ExpectedBasic: true,
			ExpectedRealm: ``,
		},

		"basic token realm": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{`basic realm=Foo Bar `},
			},
			ExpectedBasic: true,
			ExpectedRealm: `Foo Bar`,
		},

		"basic quoted realm": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{`basic realm="Foo Bar"`},
			},
			ExpectedBasic: true,
			ExpectedRealm: `Foo Bar`,
		},

		"basic case-insensitive scheme": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{`BASIC realm="Foo"`},
			},
			ExpectedBasic: true,
			ExpectedRealm: `Foo`,
		},

		"basic case-insensitive realm": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{`basic REALM="Foo"`},
			},
			ExpectedBasic: true,
			ExpectedRealm: `Foo`,
		},

		"basic whitespace": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{` 	basic 	realm 	= 	"Foo\" Bar" 	`},
			},
			ExpectedBasic: true,
			ExpectedRealm: `Foo\" Bar`,
		},

		"basic trailing comma": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{`basic realm="Foo", otherparam="value"`},
			},
			ExpectedBasic: true,
			ExpectedRealm: `Foo`,
		},

		"realm containing quotes": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{`basic realm="F\"oo", otherparam="value"`},
			},
			ExpectedBasic: true,
			ExpectedRealm: `F\"oo`,
		},

		"realm containing comma": {
			Headers: http.Header{
				WWW_AUTHENTICATE: []string{`basic realm="Foo, bar", otherparam="value"`},
			},
			ExpectedBasic: true,
			ExpectedRealm: `Foo, bar`,
		},

		// TODO: additional forms to support
		//   Basic param="value", realm="myrealm"
		//   Digest, Basic param="value", realm="myrealm"
	}

	for k, tc := range testCases {
		isBasic, realm := basicRealm(tc.Headers)
		if isBasic != tc.ExpectedBasic {
			t.Errorf("%s: Expected isBasicChallenge=%v, got %v", k, tc.ExpectedBasic, isBasic)
		}
		if realm != tc.ExpectedRealm {
			t.Errorf("%s: Expected realm=%q, got %q", k, tc.ExpectedRealm, realm)
		}
	}
}
