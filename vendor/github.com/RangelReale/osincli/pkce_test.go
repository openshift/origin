package osincli

import (
	"regexp"
	"testing"
)

var (
	pkceMatcher = regexp.MustCompile("^[a-zA-Z0-9~._-]{43,128}$")
)

func TestGeneratePKCE(t *testing.T) {
	for i := 0; i < 100; i++ {
		challenge, _, verifier, err := GeneratePKCE()
		if err != nil {
			t.Fatalf("Unexpected error: %#v", err)
		}
		if matched := pkceMatcher.MatchString(challenge); !matched {
			t.Fatalf("Invalid code challenge: %s", challenge)
		}
		if matched := pkceMatcher.MatchString(verifier); !matched {
			t.Fatalf("Invalid code verifier: %s", verifier)
		}
	}
}
