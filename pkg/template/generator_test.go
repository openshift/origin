package template

import (
	"math/rand"
	"testing"

	. "github.com/openshift/origin/pkg/template/generator"
)

func TestGeneratorTemplate(t *testing.T) {
	sampleGenerator := Generator{Seed: rand.New(rand.NewSource(1337))}

	result, _ := sampleGenerator.Generate("test[A-Z0-9]{4}template").Value()
	if result != "testQ3HVtemplate" {
		t.Errorf("Failed to process the template, result is: %s", result)
	}

	result, _ = sampleGenerator.Generate("[\\d]{4}").Value()
	if result != "6841" {
		t.Errorf("Failed to process the template, result is: %s", result)
	}

	result, _ = sampleGenerator.Generate("[\\w]{4}").Value()
	if result != "DVgK" {
		t.Errorf("Failed to process the template, result is: %s", result)
	}

	result, _ = sampleGenerator.Generate("[\\a]{10}").Value()
	if result != "nFWmvmjuaZ" {
		t.Errorf("Failed to process the template, result is: %s", result)
	}

	result, err := sampleGenerator.Generate("[GET:http://external.api.int/new]").Value()
	if err == nil {
		t.Errorf("No error returned when the HTTP request failed.")
	}
}
