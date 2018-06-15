package osin

import (
	"net/url"
	"strings"
	"testing"
)

func TestGetRedirectUrl(t *testing.T) {
	// Make sure we can round-trip state parameters containing special URL characters, both as a query param and in an encoded fragment
	state := `{"then": "/index.html?a=1&b=%2B#fragment", "nonce": "014f:bff9a07c"}`

	testcases := map[string]struct {
		URL                string
		Output             ResponseData
		RedirectInFragment bool

		ExpectedURL string
	}{
		"query": {
			URL:         "https://foo.com/path?abc=123",
			Output:      ResponseData{"access_token": "12345", "state": state},
			ExpectedURL: "https://foo.com/path?abc=123&access_token=12345&state=%7B%22then%22%3A+%22%2Findex.html%3Fa%3D1%26b%3D%252B%23fragment%22%2C+%22nonce%22%3A+%22014f%3Abff9a07c%22%7D",
		},

		// https://tools.ietf.org/html/rfc6749#section-4.2.2
		// Fragment should be encoded as application/x-www-form-urlencoded (%-escaped, spaces are represented as '+')
		"fragment": {
			URL:                "https://foo.com/path?abc=123",
			Output:             ResponseData{"access_token": "12345", "state": state},
			RedirectInFragment: true,
			ExpectedURL:        "https://foo.com/path?abc=123#access_token=12345&state=%7B%22then%22%3A+%22%2Findex.html%3Fa%3D1%26b%3D%252B%23fragment%22%2C+%22nonce%22%3A+%22014f%3Abff9a07c%22%7D",
		},
	}

	for k, tc := range testcases {
		resp := &Response{
			Type:               REDIRECT,
			URL:                tc.URL,
			Output:             tc.Output,
			RedirectInFragment: tc.RedirectInFragment,
		}
		result, err := resp.GetRedirectUrl()
		if err != nil {
			t.Errorf("%s: %v", k, err)
			continue
		}
		if result != tc.ExpectedURL {
			t.Errorf("%s: expected\n\t%v, got\n\t%v", k, tc.ExpectedURL, result)
			continue
		}

		var params url.Values
		if tc.RedirectInFragment {
			params, err = url.ParseQuery(strings.SplitN(result, "#", 2)[1])
			if err != nil {
				t.Errorf("%s: %v", k, err)
				continue
			}
		} else {
			parsedResult, err := url.Parse(result)
			if err != nil {
				t.Errorf("%s: %v", k, err)
				continue
			}
			params = parsedResult.Query()
		}

		if params["state"][0] != state {
			t.Errorf("%s: expected\n\t%v, got\n\t%v", k, state, params["state"][0])
			continue
		}
	}
}
