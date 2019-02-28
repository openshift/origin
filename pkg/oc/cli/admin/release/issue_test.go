package release

import (
	"testing"
)

func TestIssueFromURI(t *testing.T) {
	for _, testCase := range []struct {
		uri      string
		expected issue
	}{
		{
			uri: "https://bugzilla.redhat.com/123",
			expected: issue{
				Store: "rhbz",
				ID:    123,
				URI:   "https://bugzilla.redhat.com/123",
			},
		},
		{
			uri: "https://bugzilla.redhat.com/show_bug.cgi?id=123",
			expected: issue{
				Store: "rhbz",
				ID:    123,
				URI:   "https://bugzilla.redhat.com/show_bug.cgi?id=123",
			},
		},
		{
			uri: "https://github.com/openshift/origin/issues/123",
			expected: issue{
				Store: "origin",
				ID:    123,
				URI:   "https://github.com/openshift/origin/issues/123",
			},
		},
		{
			uri: "https://github.com/openshift/origin/pull/123",
			expected: issue{
				Store: "origin",
				ID:    123,
				URI:   "https://github.com/openshift/origin/pull/123",
			},
		},
		{
			uri: "https://github.com/example/repo/issues/123",
			expected: issue{
				Store: "example/repo",
				ID:    123,
				URI:   "https://github.com/example/repo/issues/123",
			},
		},
		{
			uri: "https://github.com/example/repo/pull/123",
			expected: issue{
				Store: "example/repo",
				ID:    123,
				URI:   "https://github.com/example/repo/pull/123",
			},
		},
	} {
		t.Run(testCase.uri, func(t *testing.T) {
			actual, err := issueFromURI(testCase.uri)
			if err != nil {
				t.Fatal(err)
			}

			if *actual != testCase.expected {
				t.Fatalf("unexpected result: %#+v (expected %#+v)", *actual, testCase.expected)
			}
		})
	}
}
