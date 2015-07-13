package util

import (
	"testing"
)

func TestIsPotentialRootUser(t *testing.T) {
	tests := []struct {
		str      string
		expected bool
	}{
		{
			str:      "145",
			expected: false,
		},
		{
			str:      "foo",
			expected: true,
		},
		{
			str:      "12root",
			expected: true,
		},
		{
			str:      "0",
			expected: true,
		},
		{
			str:      "",
			expected: true,
		},
	}

	for _, test := range tests {
		if a, e := IsPotentialRootUser(test.str), test.expected; a != e {
			t.Errorf("Unexpected result for %q: %v", test.str, a)
		}
	}
}

func TestIncludesRootUserDirective(t *testing.T) {
	tests := []struct {
		cmds     []string
		expected bool
	}{
		{
			cmds:     []string{"COPY test.file test.dest", "RUN script.sh", "VOLUME test test/data"},
			expected: false,
		},
		{
			cmds:     []string{"COPY test.file test.dest", "RUN script.sh", "USER root"},
			expected: true,
		},
		{
			cmds:     []string{"USER 0", "COPY test.file test.dest", "RUN script.sh"},
			expected: true,
		},
		{
			cmds:     []string{"USER 100", "COPY test.file test.dest", "RUN script.sh"},
			expected: false,
		},
		{
			cmds:     []string{"USER\t\t   100", "COPY test.file test.dest", "RUN script.sh"},
			expected: false,
		},
		{
			cmds:     []string{"USER\t\t   0", "COPY test.file test.dest", "RUN script.sh"},
			expected: true,
		},
	}

	for _, test := range tests {
		if a, e := IncludesRootUserDirective(test.cmds), test.expected; a != e {
			t.Errorf("Unexpected result for %#v: %v", test.cmds, a)
		}
	}
}
