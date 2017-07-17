package util

import (
	"testing"
)

func TestStripProxyCredentials(t *testing.T) {

	inputs := []string{
		// values w/o protocols are untouched
		"http_proxy=user:password@hostname.com",
		"https_proxy=user:password@hostname.com",
		"HTTP_PROXY=user:password@hostname.com",
		"HTTPS_PROXY=user:password@hostname.com",
		// values with protocol are properly stripped
		"http_proxy=http://user:password@hostname.com",
		"https_proxy=https://user:password@hostname.com",
		"HTTP_PROXY=http://user:password@hostname.com",
		"HTTPS_PROXY=https://user:password@hostname.com",
		"http_proxy=http://user:password@hostname.com:80",
		"https_proxy=https://user:password@hostname.com:443",
		"HTTP_PROXY=http://user:password@hostname.com:8080",
		"HTTPS_PROXY=https://user:password@hostname.com:8443",
		// values with no user info are untouched
		"http_proxy=http://hostname.com",
		"https_proxy=https://hostname.com",
		"HTTP_PROXY=http://hostname.com",
		"HTTPS_PROXY=https://hostname.com",
		// keys that don't contain "proxy" are untouched
		"othervalue=http://user:password@hostname.com",
		"othervalue=user:password@hostname.com",
		// unparseable url
		"proxy=https://user:password@foo%$ @bar@blah.com",
	}

	expected := []string{
		"http_proxy=user:password@hostname.com",
		"https_proxy=user:password@hostname.com",
		"HTTP_PROXY=user:password@hostname.com",
		"HTTPS_PROXY=user:password@hostname.com",
		"http_proxy=http://redacted@hostname.com",
		"https_proxy=https://redacted@hostname.com",
		"HTTP_PROXY=http://redacted@hostname.com",
		"HTTPS_PROXY=https://redacted@hostname.com",
		"http_proxy=http://redacted@hostname.com:80",
		"https_proxy=https://redacted@hostname.com:443",
		"HTTP_PROXY=http://redacted@hostname.com:8080",
		"HTTPS_PROXY=https://redacted@hostname.com:8443",
		"http_proxy=http://redacted@hostname.com",
		"https_proxy=https://redacted@hostname.com",
		"HTTP_PROXY=http://redacted@hostname.com",
		"HTTPS_PROXY=https://redacted@hostname.com",
		"othervalue=http://user:password@hostname.com",
		"othervalue=user:password@hostname.com",
		"proxy=https://user:password@foo%$ @bar@blah.com",
	}
	result := SafeForLoggingEnv(inputs)
	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("expected %s to be stripped to %s, but got %s", inputs[i], expected[i], result[i])
		}
	}
}
