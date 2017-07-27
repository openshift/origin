package create

import (
	"reflect"
	"testing"
)

func TestParseAnnotationSelector(t *testing.T) {
	tests := []struct {
		input         string
		parsed        map[string]string
		errorExpected bool
	}{
		{
			input:  "",
			parsed: nil,
		},
		{
			input: "foo=bar",
			parsed: map[string]string{
				"foo": "bar",
			},
		},
		{
			input: "deads=deads,liggitt=liggitt",
			parsed: map[string]string{
				"deads":   "deads",
				"liggitt": "liggitt",
			},
		},
		{
			input:         "liggitt=liggitt,deadliggitt",
			errorExpected: true,
		},
		{
			input: `"us=deads,liggitt,ffranz"`,
			parsed: map[string]string{
				"us": "deads,liggitt,ffranz",
			},
		},
		{
			input: `"us=deads,liggitt,ffranz",deadliggitt=deadliggitt`,
			parsed: map[string]string{
				"us":          "deads,liggitt,ffranz",
				"deadliggitt": "deadliggitt",
			},
		},
	}
	for _, test := range tests {
		parsed, err := parseAnnotationSelector(test.input)
		if err != nil {
			if !test.errorExpected {
				t.Fatalf("unexpected error: %s", err)
			}
			continue
		}
		if test.errorExpected {
			t.Fatalf("expected error, got a parsed output: %q", parsed)
		}
		if !reflect.DeepEqual(parsed, test.parsed) {
			t.Error("expected parsed annotation selector ", test.parsed, ", got ", parsed)
		}
	}
}
