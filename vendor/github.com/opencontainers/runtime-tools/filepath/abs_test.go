package filepath

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAbs(t *testing.T) {
	for _, test := range []struct {
		os       string
		path     string
		cwd      string
		expected string
	}{
		{
			os:       "linux",
			path:     "/",
			cwd:      "/cwd",
			expected: "/",
		},
		{
			os:       "linux",
			path:     "/a",
			cwd:      "/cwd",
			expected: "/a",
		},
		{
			os:       "linux",
			path:     "/a/",
			cwd:      "/cwd",
			expected: "/a",
		},
		{
			os:       "linux",
			path:     "//a",
			cwd:      "/cwd",
			expected: "/a",
		},
		{
			os:       "linux",
			path:     ".",
			cwd:      "/cwd",
			expected: "/cwd",
		},
		{
			os:       "linux",
			path:     "./c",
			cwd:      "/a/b",
			expected: "/a/b/c",
		},
		{
			os:       "linux",
			path:     ".//c",
			cwd:      "/a/b",
			expected: "/a/b/c",
		},
		{
			os:       "linux",
			path:     "../a",
			cwd:      "/cwd",
			expected: "/a",
		},
		{
			os:       "linux",
			path:     "../../b",
			cwd:      "/cwd",
			expected: "/b",
		},
		{
			os:       "windows",
			path:     "c:\\",
			cwd:      "/cwd",
			expected: "c:\\",
		},
		{
			os:       "windows",
			path:     "c:\\a",
			cwd:      "c:\\cwd",
			expected: "c:\\a",
		},
		{
			os:       "windows",
			path:     "c:\\a\\",
			cwd:      "c:\\cwd",
			expected: "c:\\a",
		},
		{
			os:       "windows",
			path:     "c:\\\\a",
			cwd:      "c:\\cwd",
			expected: "c:\\a",
		},
		{
			os:       "windows",
			path:     ".",
			cwd:      "c:\\cwd",
			expected: "c:\\cwd",
		},
		{
			os:       "windows",
			path:     ".\\c",
			cwd:      "c:\\a\\b",
			expected: "c:\\a\\b\\c",
		},
		{
			os:       "windows",
			path:     ".\\\\c",
			cwd:      "c:\\a\\b",
			expected: "c:\\a\\b\\c",
		},
		{
			os:       "windows",
			path:     "..\\a",
			cwd:      "c:\\cwd",
			expected: "c:\\a",
		},
		{
			os:       "windows",
			path:     "..\\..\\b",
			cwd:      "c:\\cwd",
			expected: "c:\\b",
		},
	} {
		t.Run(
			fmt.Sprintf("Abs(%q,%q,%q)", test.os, test.path, test.cwd),
			func(t *testing.T) {
				abs, err := Abs(test.os, test.path, test.cwd)
				if err != nil {
					t.Error(err)
				} else if abs != test.expected {
					t.Errorf("unexpected result: %q (expected %q)", abs, test.expected)
				}
			},
		)
	}
}

func TestIsAbs(t *testing.T) {
	for _, test := range []struct {
		os       string
		path     string
		expected bool
	}{
		{
			os:       "linux",
			path:     "/",
			expected: true,
		},
		{
			os:       "linux",
			path:     "/a",
			expected: true,
		},
		{
			os:       "linux",
			path:     "//",
			expected: true,
		},
		{
			os:       "linux",
			path:     "//a",
			expected: true,
		},
		{
			os:       "linux",
			path:     ".",
			expected: false,
		},
		{
			os:       "linux",
			path:     "./a",
			expected: false,
		},
		{
			os:       "linux",
			path:     ".//a",
			expected: false,
		},
		{
			os:       "linux",
			path:     "../a",
			expected: false,
		},
		{
			os:       "linux",
			path:     "../../a",
			expected: false,
		},
		{
			os:       "windows",
			path:     "c:\\",
			expected: true,
		},
		{
			os:       "windows",
			path:     "c:\\a",
			expected: true,
		},
		{
			os:       "windows",
			path:     ".",
			expected: false,
		},
		{
			os:       "windows",
			path:     ".\\a",
			expected: false,
		},
		{
			os:       "windows",
			path:     "..\\a",
			expected: false,
		},
	} {
		t.Run(
			fmt.Sprintf("IsAbs(%q,%q)", test.os, test.path),
			func(t *testing.T) {
				abs := IsAbs(test.os, test.path)
				if abs != test.expected {
					t.Errorf("unexpected result: %t (expected %t)", abs, test.expected)
				}
				if runtime.GOOS == test.os {
					stdAbs := filepath.IsAbs(test.path)
					if abs != stdAbs {
						t.Errorf("non-standard result: %t (%t is standard)", abs, stdAbs)
					}
				}
			},
		)
	}
}
