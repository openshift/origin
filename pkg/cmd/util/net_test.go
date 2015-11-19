package util

import (
	"reflect"
	"testing"
)

func TestHostnameMatchSpecCandidates(t *testing.T) {
	testcases := []struct {
		Hostname      string
		ExpectedSpecs []string
	}{
		{
			Hostname:      "",
			ExpectedSpecs: nil,
		},
		{
			Hostname:      "a",
			ExpectedSpecs: []string{"a", "*"},
		},
		{
			Hostname:      "foo.bar",
			ExpectedSpecs: []string{"foo.bar", "*.bar", "*.*"},
		},
	}

	for _, tc := range testcases {
		specs := HostnameMatchSpecCandidates(tc.Hostname)
		if !reflect.DeepEqual(specs, tc.ExpectedSpecs) {
			t.Errorf("%s: Expected %#v, got %#v", tc.Hostname, tc.ExpectedSpecs, specs)
		}
	}
}

func TestHostnameMatches(t *testing.T) {
	testcases := []struct {
		Hostname      string
		Spec          string
		ExpectedMatch bool
	}{
		// Empty hostname matches nothing
		{Hostname: "", Spec: "", ExpectedMatch: false},

		// Empty spec matches nothing
		{Hostname: "a", Spec: "", ExpectedMatch: false},

		// Exact match
		{Hostname: "a", Spec: "a", ExpectedMatch: true},
		// Single segment wildcard match
		{Hostname: "a", Spec: "*", ExpectedMatch: true},

		// Mismatched segment count should not match
		{Hostname: "a", Spec: "*.a", ExpectedMatch: false},
		{Hostname: "a", Spec: "*.*", ExpectedMatch: false},

		// Exact match, multi-segment
		{Hostname: "a.b", Spec: "a.b", ExpectedMatch: true},
		// Wildcard subdomain match
		{Hostname: "a.b", Spec: "*.b", ExpectedMatch: true},
		// Multi-level wildcard match
		{Hostname: "a.b", Spec: "*.*", ExpectedMatch: true},

		// Only subdomain wildcards are allowed
		{Hostname: "a.b", Spec: "a.*", ExpectedMatch: false},
		// Mismatched segment count should not match
		{Hostname: "a.b", Spec: "*.a.b", ExpectedMatch: false},
	}

	for i, tc := range testcases {
		matches := HostnameMatches(tc.Hostname, tc.Spec)
		if matches != tc.ExpectedMatch {
			t.Errorf("%d: Expected match=%v, got %v (hostname=%s, specs=%v)", i, tc.ExpectedMatch, matches, tc.Hostname, tc.Spec)
		}
	}
}
