package util

import (
	"testing"
)

type envTestCase struct {
	Input  string
	Output string
}

func TestStripProxyCredentials(t *testing.T) {

	cases := []envTestCase{
		{Input: "http_proxy=user:password@hostname.com", Output: "http_proxy=user:password@hostname.com"},
		// No stripping if protocols are excluded
		{Input: "https_proxy=user:password@hostname.com", Output: "https_proxy=user:password@hostname.com"},
		{Input: "HTTP_PROXY=user:password@hostname.com", Output: "HTTP_PROXY=user:password@hostname.com"},
		{Input: "HTTPS_PROXY=user:password@hostname.com", Output: "HTTPS_PROXY=user:password@hostname.com"},
		// values with protocol are properly stripped
		{Input: "http_proxy=http://user:password@hostname.com", Output: "http_proxy=http://redacted@hostname.com"},
		{Input: "https_proxy=https://user:password@hostname.com", Output: "https_proxy=https://redacted@hostname.com"},
		{Input: "HTTP_PROXY=http://user:password@hostname.com", Output: "HTTP_PROXY=http://redacted@hostname.com"},
		{Input: "HTTPS_PROXY=https://user:password@hostname.com", Output: "HTTPS_PROXY=https://redacted@hostname.com"},
		{Input: "http_proxy=http://user:password@hostname.com:80", Output: "http_proxy=http://redacted@hostname.com:80"},
		{Input: "https_proxy=https://user:password@hostname.com:443", Output: "https_proxy=https://redacted@hostname.com:443"},
		{Input: "HTTP_PROXY=http://user:password@hostname.com:8080", Output: "HTTP_PROXY=http://redacted@hostname.com:8080"},
		{Input: "HTTPS_PROXY=https://user:password@hostname.com:8443", Output: "HTTPS_PROXY=https://redacted@hostname.com:8443"},
		// values with no user+password info are untouched
		{Input: "http_proxy=http://hostname.com", Output: "http_proxy=http://hostname.com"},
		{Input: "https_proxy=https://hostname.com", Output: "https_proxy=https://hostname.com"},
		{Input: "HTTP_PROXY=http://hostname.com", Output: "HTTP_PROXY=http://hostname.com"},
		{Input: "HTTPS_PROXY=https://hostname.com", Output: "HTTPS_PROXY=https://hostname.com"},
		{Input: "http_proxy=http://user@hostname.com", Output: "http_proxy=http://user@hostname.com"},
		{Input: "https_proxy=https://user@hostname.com", Output: "https_proxy=https://user@hostname.com"},
		{Input: "HTTP_PROXY=http://user@hostname.com", Output: "HTTP_PROXY=http://user@hostname.com"},
		{Input: "HTTPS_PROXY=https://user@hostname.com", Output: "HTTPS_PROXY=https://user@hostname.com"},
		// keys that don't contain "proxy" are untouched
		{Input: "othervalue=http://user:password@hostname.com", Output: "othervalue=http://user:password@hostname.com"},
		{Input: "othervalue=user:password@hostname.com", Output: "othervalue=user:password@hostname.com"},
		// unparseable url
		{Input: "proxy=https://user:password@foo%$ @bar@blah.com", Output: "proxy=https://user:password@foo%$ @bar@blah.com"},
	}

	inputs := make([]string, len(cases))
	expected := make([]string, len(cases))
	for i, c := range cases {
		inputs[i] = c.Input
		expected[i] = c.Output
	}
	result := SafeForLoggingEnv(inputs)
	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("expected %s for environment variable, but got %s", expected[i], result[i])
		}
	}
}
