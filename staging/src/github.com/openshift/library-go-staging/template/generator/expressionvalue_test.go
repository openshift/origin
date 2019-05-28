package generator

import (
	"math/rand"
	"testing"
)

func TestExpressionValueGenerator(t *testing.T) {
	var tests = []struct {
		Expression    string
		ExpectedValue string
	}{
		{"test[A-Z0-9]{4}template", "testQ3HVtemplate"},
		{"[\\d]{3}", "889"},
		{"[\\w]{20}", "hiG4uRbcUDd5PEJLyHZ7"},
		{"[\\a]{10}", "4U390O49B9"},
		{"[\\A]{10}", ")^&-|_:[><"},
		{"strongPassword[\\w]{3}[\\A]{3}", "strongPasswordhiG-|_"},
		{"admin[0-9]{2}[A-Z]{2}", "admin78YB"},
		{"admin[0-9]{2}test[A-Z]{2}", "admin78testYB"},
	}

	for _, test := range tests {
		generator := NewExpressionValueGenerator(rand.New(rand.NewSource(1337)))
		value, err := generator.GenerateValue(test.Expression)
		if err != nil {
			t.Errorf("Failed to generate value from %s due to error: %v", test.Expression, err)
		}
		if value != test.ExpectedValue {
			t.Errorf("Failed to generate expected value from %s\n. Generated: %s\n. Expected: %s\n", test.Expression, value, test.ExpectedValue)
		}
	}
}

func TestRemoveDuplicatedCharacters(t *testing.T) {
	var tests = []struct {
		Expression    string
		ExpectedValue string
	}{
		{"abcdefgh", "abcdefgh"},
		{"abcabc", "abc"},
		{"1111111", "1"},
		{"1234567890", "1234567890"},
		{"test@@", "tes@"},
	}

	for _, test := range tests {
		result := removeDuplicateChars(test.Expression)
		if result != test.ExpectedValue {
			t.Errorf("Expected %q, got %q", test.ExpectedValue, result)
		}
	}
}

func TestExpressionValueGeneratorErrors(t *testing.T) {
	generator := NewExpressionValueGenerator(rand.New(rand.NewSource(1337)))

	if v, err := generator.GenerateValue("[ABC]{3}"); err == nil {
		t.Errorf("Expected [ABC]{3} to produce malformed syntax error (returned: %s)", v)
	}

	if v, err := generator.GenerateValue("[Z-A]{3}"); err == nil {
		t.Errorf("Expected Invalid range specified error, got %s", v)
	}

	if v, err := generator.GenerateValue("[A-Z]{300}"); err == nil {
		t.Errorf("Expected Invalid range specified error, got %s", v)
	}

	if v, err := generator.GenerateValue("[A-Z]{0}"); err == nil {
		t.Errorf("Expected Invalid range specified error, got %s", v)
	}
}
