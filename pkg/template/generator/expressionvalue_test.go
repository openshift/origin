package generator

import (
	"math/rand"
	"testing"
)

func TestExpressionValueGenerator(t *testing.T) {
	generator := NewExpressionValueGenerator(rand.New(rand.NewSource(1337)))

	var tests = []struct {
		Expression    string
		ExpectedValue string
	}{
		{"test[A-Z0-9]{4}template", "testQ3HVtemplate"},
		{"[\\d]{4}", "6841"},
		{"[\\w]{4}", "DVgK"},
		{"[\\a]{10}", "nFWmvmjuaZ"},
		{"admin[0-9]{2}[A-Z]{2}", "admin32VU"},
		{"admin[0-9]{2}test[A-Z]{2}", "admin56testGS"},
	}

	for _, test := range tests {
		value, err := generator.GenerateValue(test.Expression)
		if err != nil {
			t.Errorf("Failed to generate value from %s due to error: %v", test.Expression, err)
		}
		if value != test.ExpectedValue {
			t.Errorf("Failed to generate expected value from %s\n. Generated: %s\n. Expected: %s\n", test.Expression, value, test.ExpectedValue)
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
