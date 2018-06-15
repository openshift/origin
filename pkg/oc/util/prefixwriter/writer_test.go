package prefixwriter

import (
	"bytes"
	"fmt"
	"testing"
)

func TestPrefixWriter(t *testing.T) {
	tests := []struct {
		input    []string
		prefix   string
		expected string
	}{
		{
			input:    []string{"hello\nworld"},
			prefix:   "--",
			expected: "--hello\n--world",
		},
		{
			input:    []string{"hello", "world", "\ntest\n", "string"},
			prefix:   "-",
			expected: "-helloworld\n-test\n-string",
		},
		{
			input:    []string{"\n"},
			prefix:   "-",
			expected: "\n",
		},
		{
			input:    []string{"hello\nworld\n"},
			prefix:   " ",
			expected: " hello\n world\n",
		},
		{
			input:    []string{"\n", "\n", "a", "b"},
			prefix:   "==> ",
			expected: "\n\n==> ab",
		},
	}

	for _, test := range tests {
		internal := &bytes.Buffer{}
		prefixWriter := New(test.prefix, internal)
		for _, line := range test.input {
			fmt.Fprintf(prefixWriter, "%s", line)
		}
		if internal.String() != test.expected {
			t.Errorf("Unexpected result.\nInput: %v\nExpected:\n%s\nActual:\n%s\n", test.input, test.expected, internal.String())
		}
	}
}
