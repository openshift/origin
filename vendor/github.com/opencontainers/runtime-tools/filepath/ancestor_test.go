package filepath

import (
	"fmt"
	"testing"
)

func TestIsAncestor(t *testing.T) {
	for _, test := range []struct {
		os       string
		pathA    string
		pathB    string
		cwd      string
		expected bool
	}{
		{
			os:       "linux",
			pathA:    "/",
			pathB:    "/a",
			cwd:      "/cwd",
			expected: true,
		},
		{
			os:       "linux",
			pathA:    "/a",
			pathB:    "/a",
			cwd:      "/cwd",
			expected: false,
		},
		{
			os:       "linux",
			pathA:    "/a",
			pathB:    "/",
			cwd:      "/cwd",
			expected: false,
		},
		{
			os:       "linux",
			pathA:    "/a",
			pathB:    "/ab",
			cwd:      "/cwd",
			expected: false,
		},
		{
			os:       "linux",
			pathA:    "/a/",
			pathB:    "/a",
			cwd:      "/cwd",
			expected: false,
		},
		{
			os:       "linux",
			pathA:    "//a",
			pathB:    "/a",
			cwd:      "/cwd",
			expected: false,
		},
		{
			os:       "linux",
			pathA:    "//a",
			pathB:    "/a/b",
			cwd:      "/cwd",
			expected: true,
		},
		{
			os:       "linux",
			pathA:    "/a",
			pathB:    "/a/",
			cwd:      "/cwd",
			expected: false,
		},
		{
			os:       "linux",
			pathA:    "/a",
			pathB:    ".",
			cwd:      "/cwd",
			expected: false,
		},
		{
			os:       "linux",
			pathA:    "/a",
			pathB:    "b",
			cwd:      "/a",
			expected: true,
		},
		{
			os:       "linux",
			pathA:    "/a",
			pathB:    "../a",
			cwd:      "/cwd",
			expected: false,
		},
		{
			os:       "linux",
			pathA:    "/a",
			pathB:    "../a/b",
			cwd:      "/cwd",
			expected: true,
		},
		{
			os:       "windows",
			pathA:    "c:\\",
			pathB:    "c:\\a",
			cwd:      "c:\\cwd",
			expected: true,
		},
		{
			os:       "windows",
			pathA:    "c:\\",
			pathB:    "d:\\a",
			cwd:      "c:\\cwd",
			expected: false,
		},
		{
			os:       "windows",
			pathA:    "c:\\",
			pathB:    ".",
			cwd:      "d:\\cwd",
			expected: false,
		},
		{
			os:       "windows",
			pathA:    "c:\\a",
			pathB:    "c:\\a",
			cwd:      "c:\\cwd",
			expected: false,
		},
		{
			os:       "windows",
			pathA:    "c:\\a",
			pathB:    "c:\\",
			cwd:      "c:\\cwd",
			expected: false,
		},
		{
			os:       "windows",
			pathA:    "c:\\a",
			pathB:    "c:\\ab",
			cwd:      "c:\\cwd",
			expected: false,
		},
		{
			os:       "windows",
			pathA:    "c:\\a\\",
			pathB:    "c:\\a",
			cwd:      "c:\\cwd",
			expected: false,
		},
		{
			os:       "windows",
			pathA:    "c:\\\\a",
			pathB:    "c:\\a",
			cwd:      "c:\\cwd",
			expected: false,
		},
		{
			os:       "windows",
			pathA:    "c:\\\\a",
			pathB:    "c:\\a\\b",
			cwd:      "c:\\cwd",
			expected: true,
		},
		{
			os:       "windows",
			pathA:    "c:\\a",
			pathB:    "c:\\a\\",
			cwd:      "c:\\cwd",
			expected: false,
		},
		{
			os:       "windows",
			pathA:    "c:\\a",
			pathB:    ".",
			cwd:      "c:\\cwd",
			expected: false,
		},
		{
			os:       "windows",
			pathA:    "c:\\a",
			pathB:    "b",
			cwd:      "c:\\a",
			expected: true,
		},
		{
			os:       "windows",
			pathA:    "c:\\a",
			pathB:    "..\\a",
			cwd:      "c:\\cwd",
			expected: false,
		},
		{
			os:       "windows",
			pathA:    "c:\\a",
			pathB:    "..\\a\\b",
			cwd:      "c:\\cwd",
			expected: true,
		},
	} {
		t.Run(
			fmt.Sprintf("IsAncestor(%q,%q,%q,%q)", test.os, test.pathA, test.pathB, test.cwd),
			func(t *testing.T) {
				ancestor, err := IsAncestor(test.os, test.pathA, test.pathB, test.cwd)
				if err != nil {
					t.Error(err)
				} else if ancestor != test.expected {
					t.Errorf("unexpected result: %t", ancestor)
				}
			},
		)
	}
}
