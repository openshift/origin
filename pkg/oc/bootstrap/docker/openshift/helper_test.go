package openshift

import (
	"testing"
)

func TestParseOpenshiftVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "1.3.1",
			expected: "1.3.1",
		},
		{
			input:    "3.5.5.4",
			expected: "3.5.5",
		},
		{
			input:    "3.5.5.18-alpha.3",
			expected: "3.5.5-alpha.3",
		},
		{
			input:    "3.4.144.2-1",
			expected: "3.4.144-1",
		},
		{
			input:    "3.6.0-alpha.2+2a00043-774",
			expected: "3.6.0-alpha.2+2a00043-774",
		},
		{
			input:    "3.6.172.0.4",
			expected: "3.6.172",
		},
	}

	for _, test := range tests {
		v, err := parseOpenshiftVersion(test.input)
		if err != nil {
			t.Errorf("unexpected error for %s: %v", test.input, err)
			continue
		}
		if v.String() != test.expected {
			t.Errorf("unexpected output. Got: %v, expected: %s", v, test.expected)
		}
	}
}
