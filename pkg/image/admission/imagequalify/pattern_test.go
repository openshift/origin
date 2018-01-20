package imagequalify_test

import (
	"reflect"
	"testing"

	"github.com/openshift/origin/pkg/image/admission/imagequalify"
)

func TestPatternDestructure(t *testing.T) {
	var testcases = []struct {
		pattern  string
		expected imagequalify.PatternParts
	}{{
		pattern: "a",
		expected: imagequalify.PatternParts{
			Path: "a",
		},
	}, {
		pattern: "a:latest",
		expected: imagequalify.PatternParts{
			Path: "a",
			Tag:  "latest",
		},
	}, {
		pattern: "a/b",
		expected: imagequalify.PatternParts{
			Depth: 1,
			Path:  "a/b",
		},
	}, {
		pattern: "a/b:latest",
		expected: imagequalify.PatternParts{
			Depth: 1,
			Path:  "a/b",
			Tag:   "latest",
		},
	}, {
		pattern: "a@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		expected: imagequalify.PatternParts{
			Digest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			Path:   "a",
		},
	}, {
		pattern: "a/b@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		expected: imagequalify.PatternParts{
			Depth:  1,
			Digest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			Path:   "a/b",
		},
	}, {
		pattern: "a:latest@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		expected: imagequalify.PatternParts{
			Digest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			Path:   "a",
			Tag:    "latest",
		},
	}, {
		pattern: "a/b:latest@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		expected: imagequalify.PatternParts{
			Depth:  1,
			Digest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			Path:   "a/b",
			Tag:    "latest",
		},
	}}

	for i, tc := range testcases {
		actual := imagequalify.DestructurePattern(tc.pattern)
		if !reflect.DeepEqual(tc.expected, actual) {
			t.Errorf("test #%v: expected %#v, got %#v", i, tc.expected, actual)
		}
	}
}
