package validation

import (
	"testing"
)

func TestImageParseDomainName(t *testing.T) {
	for i, test := range []struct {
		input     string
		domain    string
		remainder string
		expectErr bool
	}{{
		expectErr: true,
	}, {
		input:     "!baddomain!/foo",
		expectErr: true,
	}, {
		input:     "busybox",
		remainder: "busybox",
	}, {
		input:     "busybox:latest",
		remainder: "busybox:latest",
	}, {
		input:     "foo/busybox:latest",
		remainder: "foo/busybox:latest",
	}, {
		input:     "localhost:busybox",
		remainder: "localhost:busybox",
	}, {
		input:     "localhost/busybox",
		domain:    "localhost",
		remainder: "busybox",
	}, {
		input:     "localhost:5000/busybox",
		domain:    "localhost:5000",
		remainder: "busybox",
	}, {
		input:     "localhost:5000/busybox:v1.0",
		domain:    "localhost:5000",
		remainder: "busybox:v1.0",
	}, {
		input:     "localhost:5000/foo/busybox:v1.2.3",
		domain:    "localhost:5000",
		remainder: "foo/busybox:v1.2.3",
	}, {
		input:     "localhost/foo/busybox:v1.2.3",
		domain:    "localhost",
		remainder: "foo/busybox:v1.2.3",
	}, {
		input:     "parser.test.io/busybox",
		domain:    "parser.test.io",
		remainder: "busybox",
	}, {
		input:     "parser.test.io/foo/busybox",
		domain:    "parser.test.io",
		remainder: "foo/busybox",
	}, {
		input:     "parser.test.io/busybox:v1.2.3",
		domain:    "parser.test.io",
		remainder: "busybox:v1.2.3",
	}, {
		input:     "parser.test.io/foo/busybox:v1.2.3",
		domain:    "parser.test.io",
		remainder: "foo/busybox:v1.2.3",
	}} {
		t.Logf("test #%v: %s", i, test.input)
		domain, remainder, err := ParseDomainName(test.input)
		if test.expectErr && err == nil {
			t.Errorf("test %#v: expected error", i)
		} else if !test.expectErr && err != nil {
			t.Errorf("test %#v: expected no error, got %s", i, err)
		}
		if test.domain != domain {
			t.Errorf("test #%v: failed; expected %q, got %q", i, test.domain, domain)
		}
		if test.remainder != remainder {
			t.Errorf("test #%v: failed; expected %q, got %q", i, test.remainder, remainder)
		}
	}
}
