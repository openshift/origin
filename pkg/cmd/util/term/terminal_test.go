package term

import (
	"strings"
	"testing"
)

func TestReadInputFromTerminal(t *testing.T) {
	testcases := map[string]struct {
		Input  string
		Output string
	}{
		"empty":                             {},
		"empty newline":                     {Input: "\n"},
		"empty windows newline":             {Input: "\r\n"},
		"empty newline with extra":          {Input: "\nextra"},
		"leading space":                     {Input: " data", Output: " data"},
		"leading space newline":             {Input: " data\n", Output: " data"},
		"leading space windows newline":     {Input: " data\r\n", Output: " data"},
		"leading space newline with extra":  {Input: " data\nextra", Output: " data"},
		"trailing space":                    {Input: " data ", Output: " data "},
		"trailing space newline":            {Input: " data \n", Output: " data "},
		"trailing space windows newline":    {Input: " data \r\n", Output: " data "},
		"trailing space newline with extra": {Input: " data \nextra", Output: " data "},
	}

	for k, tc := range testcases {
		output := readInputFromTerminal(strings.NewReader(tc.Input))
		if output != tc.Output {
			t.Errorf("%s: Expected '%s', got '%s'", k, tc.Output, output)
		}
	}
}
