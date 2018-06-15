package osin

import (
	"testing"
)

func TestURIValidate(t *testing.T) {
	valid := [][]string{
		{
			// Exact match
			"http://localhost:14000/appauth",
			"http://localhost:14000/appauth",
		},
		{
			// Trailing slash
			"http://www.google.com/myapp",
			"http://www.google.com/myapp/",
		},
		{
			// Exact match with trailing slash
			"http://www.google.com/myapp/",
			"http://www.google.com/myapp/",
		},
		{
			// Subpath
			"http://www.google.com/myapp",
			"http://www.google.com/myapp/interface/implementation",
		},
		{
			// Subpath with trailing slash
			"http://www.google.com/myapp/",
			"http://www.google.com/myapp/interface/implementation",
		},
		{
			// Subpath with things that are close to path traversals, but aren't
			"http://www.google.com/myapp",
			"http://www.google.com/myapp/.../..implementation../...",
		},
		{
			// If the allowed basepath contains path traversals, allow them?
			"http://www.google.com/traversal/../allowed",
			"http://www.google.com/traversal/../allowed/with/subpath",
		},
	}
	for _, v := range valid {
		if err := ValidateUri(v[0], v[1]); err != nil {
			t.Errorf("Expected ValidateUri(%s, %s) to succeed, got %v", v[0], v[1], err)
		}
	}

	invalid := [][]string{
		{
			// Doesn't satisfy base path
			"http://localhost:14000/appauth",
			"http://localhost:14000/app",
		},
		{
			// Doesn't satisfy base path
			"http://localhost:14000/app/",
			"http://localhost:14000/app",
		},
		{
			// Not a subpath of base path
			"http://localhost:14000/appauth",
			"http://localhost:14000/appauthmodifiedpath",
		},
		{
			// Host mismatch
			"http://www.google.com/myapp",
			"http://www2.google.com/myapp",
		},
		{
			// Scheme mismatch
			"http://www.google.com/myapp",
			"https://www.google.com/myapp",
		},
		{
			// Path traversal
			"http://www.google.com/myapp",
			"http://www.google.com/myapp/..",
		},
		{
			// Embedded path traversal
			"http://www.google.com/myapp",
			"http://www.google.com/myapp/../test",
		},
		{
			// Not a subpath
			"http://www.google.com/myapp",
			"http://www.google.com/myapp../test",
		},
	}
	for _, v := range invalid {
		if err := ValidateUri(v[0], v[1]); err == nil {
			t.Errorf("Expected ValidateUri(%s, %s) to fail", v[0], v[1])
		}
	}
}

func TestURIListValidate(t *testing.T) {
	// V1
	if err := ValidateUriList("http://localhost:14000/appauth", "http://localhost:14000/appauth", ""); err != nil {
		t.Errorf("V1: %s", err)
	}

	// V2
	if err := ValidateUriList("http://localhost:14000/appauth", "http://localhost:14000/app", ""); err == nil {
		t.Error("V2 should have failed")
	}

	// V3
	if err := ValidateUriList("http://xxx:14000/appauth;http://localhost:14000/appauth", "http://localhost:14000/appauth", ";"); err != nil {
		t.Errorf("V3: %s", err)
	}

	// V4
	if err := ValidateUriList("http://xxx:14000/appauth;http://localhost:14000/appauth", "http://localhost:14000/app", ";"); err == nil {
		t.Error("V4 should have failed")
	}
}
