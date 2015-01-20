package api

import (
	"testing"
)

func TestSplitDockerPullSpec(t *testing.T) {
	testCases := []struct {
		From                           string
		Registry, Namespace, Name, Tag string
		Err                            bool
	}{
		{
			From: "foo",
			Name: "foo",
		},
		{
			From:      "bar/foo",
			Namespace: "bar",
			Name:      "foo",
		},
		{
			From:      "bar/foo/baz",
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
		},
		{
			From:      "bar/foo/baz:tag",
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			Tag:       "tag",
		},
		{
			From:      "bar:5000/foo/baz:tag",
			Registry:  "bar:5000",
			Namespace: "foo",
			Name:      "baz",
			Tag:       "tag",
		},
		{
			From: "bar/foo/baz/biz",
			Err:  true,
		},
		{
			From: "",
			Err:  true,
		},
	}

	for _, testCase := range testCases {
		r, ns, n, tag, err := SplitDockerPullSpec(testCase.From)
		switch {
		case err != nil && !testCase.Err:
			t.Errorf("%s: unexpected error: %v", testCase.From, err)
			continue
		case err == nil && testCase.Err:
			t.Errorf("%s: unexpected non-error", testCase.From)
			continue
		}
		if r != testCase.Registry || ns != testCase.Namespace || n != testCase.Name || tag != testCase.Tag {
			t.Errorf("%s: unexpected result: %q %q %q %q", testCase.From, r, ns, n, tag)
		}
	}
}
