package helpers

import (
	"fmt"
	"testing"
)

func TestResolvePaths(t *testing.T) {
	tests := []struct {
		ref, base, expected string
	}{
		{"", "/foo", ""},
		{"-", "/foo", "-"},
		{"bar", "/foo", "/foo/bar"},
		{"..", "/foo", "/"},
		{"/bar", "/foo", "/bar"},
		{"bar/-", "/foo", "/foo/bar/-"},
		{"./-", "/foo", "/foo/-"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s onto %s", tt.ref, tt.base), func(t *testing.T) {
			x := tt.ref
			if err := ResolvePaths([]*string{&x}, tt.base); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if x != tt.expected {
				t.Errorf("unexpected result %q, expected %q", x, tt.expected)
			}
		})
	}
}

func TestRelativizePathWithNoBacksteps(t *testing.T) {
	tests := []struct {
		ref, base, expected string
	}{
		{"/foo/", "/foo", "."},
		{"-", "/foo", "-"},
		{"/foo/bar", "/foo", "bar"},
		{"/abc", "/foo", "/abc"},
		{"/foo/-", "/foo", "./-"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s onto %s", tt.ref, tt.base), func(t *testing.T) {
			x := tt.ref
			if err := RelativizePathWithNoBacksteps([]*string{&x}, tt.base); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if x != tt.expected {
				t.Errorf("unexpected result %q, expected %q", x, tt.expected)
			}
		})
	}
}
