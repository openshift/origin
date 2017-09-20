package app

import (
	"reflect"
	"testing"

	"github.com/openshift/origin/pkg/api/apihelpers"

	kvalidation "k8s.io/apimachinery/pkg/util/validation"
)

func TestUniqueNameGeneratorNameRequired(t *testing.T) {
	nameGenerator := NewUniqueNameGenerator("")
	_, err := nameGenerator.Generate(&ImageRef{})
	if err != ErrNameRequired {
		t.Errorf("err = %#v; want %#v", err, ErrNameRequired)
	}
}

func TestUniqueNameGeneratorEnsureValidName(t *testing.T) {
	chars := []byte("abcdefghijk")
	longBytes := []byte{}
	for i := 0; i < (kvalidation.DNS1123SubdomainMaxLength + 20); i++ {
		longBytes = append(longBytes, chars[i%len(chars)])
	}
	longName := string(longBytes)
	tests := []struct {
		name        string
		input       []string
		expected    []string
		expectError bool
	}{
		{
			name:     "duplicate names",
			input:    []string{"one", "two", "three", "one", "one", "two"},
			expected: []string{"one", "two", "three", "one-1", "one-2", "two-1"},
		},
		{
			name:     "mixed case names",
			input:    []string{"One", "ONE", "tWo"},
			expected: []string{"one", "one-1", "two"},
		},
		{
			name:     "non-standard characters",
			input:    []string{"Emby.One", "test-_test", "_-_", "@-MyRepo"},
			expected: []string{"embyone", "test-test", "", "myrepo"},
		},
		{
			name:        "short name",
			input:       []string{"t"},
			expectError: true,
		},
		{
			name:  "long name",
			input: []string{longName, longName, longName},
			expected: []string{longName[:kvalidation.DNS1123SubdomainMaxLength],
				apihelpers.GetName(longName[:kvalidation.DNS1123SubdomainMaxLength], "1", kvalidation.DNS1123SubdomainMaxLength),
				apihelpers.GetName(longName[:kvalidation.DNS1123SubdomainMaxLength], "2", kvalidation.DNS1123SubdomainMaxLength),
			},
		},
	}

tests:
	for _, test := range tests {
		result := []string{}
		nameGenerator := NewUniqueNameGenerator("").(*uniqueNameGenerator)
		for _, i := range test.input {
			name, err := nameGenerator.ensureValidName(i)
			if err != nil && !test.expectError {
				t.Errorf("%s: unexpected error: %v", test.name, err)
			}
			if err == nil && test.expectError {
				t.Errorf("%s: did not get an error.", test.name)
			}
			if err != nil {
				continue tests
			}
			result = append(result, name)
		}
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("%s: unexpected output. Expected: %#v, Got: %#v", test.name, test.expected, result)
		}
	}
}
