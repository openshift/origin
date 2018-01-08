package validation

import (
	"strings"
	"testing"
)

func TestDomainNameValid(t *testing.T) {
	for i, domain := range []string{
		"test.io",
		"localhost",
		"localhost:5000",
		"a.b.c.d.e.f",
		"a.b.c.d.e.f:5000",
	} {
		if err := validateDomain(domain); err != nil {
			t.Errorf("test #%d: unexpected error for %q, got %v", i, domain, err)
		}
	}
}

func TestDomainNameErrors(t *testing.T) {
	for i, test := range []struct {
		description string
		input       string
	}{{
		description: "empty input",
		input:       "",
	}, {
		description: "bad characters in domain name",
		input:       "!invalidname!",
	}, {
		description: "no '.' or :<PORT> and not 'localhost'",
		input:       "domain",
	}, {
		description: "name too long",
		input:       strings.Repeat("x", 255) + ".io",
	}} {
		if err := validateDomain(test.input); err == nil {
			t.Errorf("test #%v: expected error", i)
		}
	}
}
