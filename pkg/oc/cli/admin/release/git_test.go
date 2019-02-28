package release

import (
	"reflect"
	"strings"
	"testing"
)

func TestCommitProcess(t *testing.T) {
	for _, testCase := range []struct {
		initial  commit
		body     string
		expected commit
	}{
		{
			body:     "no trailers",
			initial:  commit{},
			expected: commit{},
		},
		{
			body: "pull request subject",
			initial: commit{
				Parents: []string{"hash1", "hash2"},
				Subject: "Merge pull request #123 from example",
			},
			expected: commit{
				Parents:     []string{"hash1", "hash2"},
				Subject:     "pull request subject",
				PullRequest: 123,
			},
		},
		{
			body: "subject bug reference",
			initial: commit{
				Subject: "Bug 123: example commit",
			},
			expected: commit{
				Subject: "example commit",
				Issues: []*issue{
					{
						Store: "rhbz",
						ID:    123,
						URI:   "https://bugzilla.redhat.com/show_bug.cgi?id=123",
					},
				},
			},
		},
		{
			body:    "trailer bug references\n\nIssue: https://github.com/openshift/origin/issues/123\nIssue: https://github.com/example/repo/pull/456\n",
			initial: commit{},
			expected: commit{
				Issues: []*issue{
					{
						Store: "origin",
						ID:    123,
						URI:   "https://github.com/openshift/origin/issues/123",
					},
					{
						Store: "example/repo",
						ID:    456,
						URI:   "https://github.com/example/repo/pull/456",
					},
				},
			},
		},
		{
			body: "subject and trailer bug references\n\nIssue: https://github.com/openshift/origin/issues/123\nIssue: https://github.com/example/repo/pull/456\n",
			initial: commit{
				Subject: "Bug 123: example commit",
			},
			expected: commit{
				Subject: "example commit",
				Issues: []*issue{
					{
						Store: "rhbz",
						ID:    123,
						URI:   "https://bugzilla.redhat.com/show_bug.cgi?id=123",
					},
					{
						Store: "origin",
						ID:    123,
						URI:   "https://github.com/openshift/origin/issues/123",
					},
					{
						Store: "example/repo",
						ID:    456,
						URI:   "https://github.com/example/repo/pull/456",
					},
				},
			},
		},
	} {
		name := strings.SplitN(testCase.body, "\n", 2)[0]
		t.Run(name, func(t *testing.T) {
			cmt := &testCase.initial
			err := cmt.process(testCase.body)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(cmt, &testCase.expected) {
				t.Fatalf("unexpected result: %#+v (expected %#+v)", *cmt, testCase.expected)
			}
		})
	}
}
