package spnego

import (
	"net/http"
	"testing"

	"github.com/apcera/gssapi"
)

func TestCheckSPNEGONegotiate(t *testing.T) {
	lib, err := gssapi.Load(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	name := "WWW-Authenticate"
	canonicalName := http.CanonicalHeaderKey(name)

	testcases := map[string]struct {
		Headers         http.Header
		Name            string
		ExpectedPresent bool
		ExpectedToken   string
	}{
		"empty": {
			Headers:         http.Header{},
			Name:            name,
			ExpectedPresent: false,
			ExpectedToken:   "",
		},

		"non-negotiate": {
			Headers:         http.Header{canonicalName: []string{"Basic"}},
			Name:            name,
			ExpectedPresent: false,
			ExpectedToken:   "",
		},

		"negotiate, no token": {
			Headers:         http.Header{canonicalName: []string{"Negotiate"}},
			Name:            name,
			ExpectedPresent: true,
			ExpectedToken:   "",
		},
		"negotiate, case-insensitive": {
			Headers:         http.Header{canonicalName: []string{"negotiate"}},
			Name:            name,
			ExpectedPresent: true,
			ExpectedToken:   "",
		},
		"negotiate, fallback from basic-auth": {
			Headers:         http.Header{canonicalName: []string{"Basic", "Negotiate"}},
			Name:            name,
			ExpectedPresent: true,
			ExpectedToken:   "",
		},

		"negotiate, with token": {
			Headers:         http.Header{canonicalName: []string{"Negotiate aGVsbG8="}},
			Name:            name,
			ExpectedPresent: true,
			ExpectedToken:   "hello",
		},
		"negotiate, with token with whitespace": {
			Headers:         http.Header{canonicalName: []string{"Negotiate    aGVs bG8="}},
			Name:            name,
			ExpectedPresent: true,
			ExpectedToken:   "hello",
		},

		"negotiate, with token needing no padding": {
			Headers:         http.Header{canonicalName: []string{"Negotiate cGFk"}},
			Name:            name,
			ExpectedPresent: true,
			ExpectedToken:   "pad",
		},
		"negotiate, with token with 1 end-padding =": {
			Headers:         http.Header{canonicalName: []string{"Negotiate cGFkXzE="}},
			Name:            name,
			ExpectedPresent: true,
			ExpectedToken:   "pad_1",
		},
		"negotiate, with token missing 1 end-padding =": {
			Headers:         http.Header{canonicalName: []string{"Negotiate cGFkXzE"}},
			Name:            name,
			ExpectedPresent: true,
			ExpectedToken:   "pad_1",
		},
		"negotiate, with token with 2 end-padding =": {
			Headers:         http.Header{canonicalName: []string{"Negotiate cGFkX19fMg=="}},
			Name:            name,
			ExpectedPresent: true,
			ExpectedToken:   "pad___2",
		},
		"negotiate, with token missing 2 end-padding =": {
			Headers:         http.Header{canonicalName: []string{"Negotiate cGFkX19fMg"}},
			Name:            name,
			ExpectedPresent: true,
			ExpectedToken:   "pad___2",
		},

		"negotiate, with invalid token": {
			Headers:         http.Header{canonicalName: []string{"Negotiate !@#$%"}},
			Name:            name,
			ExpectedPresent: false,
			ExpectedToken:   "",
		},
	}

	for k, tc := range testcases {
		present, token := CheckSPNEGONegotiate(lib, tc.Headers, tc.Name)
		if present != tc.ExpectedPresent {
			t.Errorf("%s: expected present=%v, got %v", k, tc.ExpectedPresent, present)
			continue
		}
		if token.String() != tc.ExpectedToken {
			t.Errorf("%s: expected token=%q, got %q", k, tc.ExpectedToken, token)
			continue
		}
	}
}
