package registryclient

import (
	"net/url"
	"testing"
)

func TestBasicCredentials(t *testing.T) {
	creds := NewBasicCredentials()
	creds.Add(&url.URL{Host: "localhost"}, "test", "other")
	if u, p := creds.Basic(&url.URL{Host: "test"}); u != "" || p != "" {
		t.Fatalf("unexpected response: %s %s", u, p)
	}
	if u, p := creds.Basic(&url.URL{Host: "localhost"}); u != "test" || p != "other" {
		t.Fatalf("unexpected response: %s %s", u, p)
	}
	if u, p := creds.Basic(&url.URL{Host: "localhost", Path: "/foo"}); u != "test" || p != "other" {
		t.Fatalf("unexpected response: %s %s", u, p)
	}
}
