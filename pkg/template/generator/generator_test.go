package generator

import (
	"math/rand"
	"testing"

	generator "."
)

func TestCreateGenerator(t *testing.T) {
	_, err := generator.New(rand.New(rand.NewSource(1337)))
	if err != nil {
		t.Errorf("Failed to create generator")
	}
}

func TestExpressionValueGenerator(t *testing.T) {
	sampleGenerator, _ := generator.New(rand.New(rand.NewSource(1337)))

	var tests = []struct {
		Expression    string
		ExpectedValue string
	}{
		{"test[A-Z0-9]{4}template", "testQ3HVtemplate"},
		{"[\\d]{4}", "6841"},
		{"[\\w]{4}", "DVgK"},
		{"[\\a]{10}", "nFWmvmjuaZ"},
	}

	for _, test := range tests {
		value, _ := sampleGenerator.GenerateValue(test.Expression)
		if value != test.ExpectedValue {
			t.Errorf("Failed to generate expected value from %s\n. Generated: %s\n. Expected: %s\n", test.Expression, value, test.ExpectedValue)
		}
	}
}

func TestPasswordGenerator(t *testing.T) {
	sampleGenerator, _ := generator.New(rand.New(rand.NewSource(1337)))

	value, _ := sampleGenerator.GenerateValue("password")
	if value != "4U390O49" {
		t.Errorf("Failed to generate expected password. Generated: %s\n. Expected: %s\n", value, "4U390O49")
	}
}

func TestRemoteValueGenerator(t *testing.T) {
	sampleGenerator, _ := generator.New(rand.New(rand.NewSource(1337)))

	_, err := sampleGenerator.GenerateValue("[GET:http://api.example.com/new]")
	if err == nil {
		t.Errorf("Expected error while fetching non-existent remote.")
	}
}
