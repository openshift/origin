package controller

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
