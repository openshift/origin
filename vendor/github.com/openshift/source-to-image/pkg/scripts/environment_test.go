package scripts

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/builder/dockerfile"
	"github.com/openshift/source-to-image/pkg/api"
)

func TestConvertEnvironmentList(t *testing.T) {
	testEnv := api.EnvironmentList{
		{Name: "Key1", Value: "Value1"},
		{Name: "Key2", Value: "Value2"},
		{Name: "Key3", Value: "Value3"},
		{Name: "Key4", Value: "Value=4"},
		{Name: "Key5", Value: "Value,5"},
	}
	result := ConvertEnvironmentList(testEnv)
	expected := []string{"Key1=Value1", "Key2=Value2", "Key3=Value3", "Key4=Value=4", "Key5=Value,5"}
	if !equalArrayContents(result, expected) {
		t.Errorf("Unexpected result. Expected: %#v. Actual: %#v",
			expected, result)
	}
}

func equalArrayContents(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, e := range a {
		found := false
		for _, f := range b {
			if f == e {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestConvertEnvironmentToDocker(t *testing.T) {
	testValues := []string{
		"Value1",
		"$Value1",
		"${Value1}",
		"`Value1`",
		`"`,
	}
	var charValues []string
	for ch := rune(32); ch < 127; ch++ {
		charValues = append(charValues, "AB"+string(ch)+"CD", "AB\\"+string(ch)+"CD")
	}
	for _, testValue := range append(testValues, charValues...) {
		list := api.EnvironmentList{{Name: "TEST", Value: testValue}}
		converted := ConvertEnvironmentToDocker(list)
		config, err := dockerfile.BuildFromConfig(&container.Config{}, []string{converted})
		if err != nil {
			t.Fatalf("Unexpected error building using Dockerfile contents %v: %v", converted, err)
		}
		if len(config.Env) != 1 {
			t.Errorf("Unexpected result. Expected 1 environment variable, got config %#v", config)
		}
		if config.Env[0] != "TEST="+testValue {
			t.Errorf("Unexpected result. Expected: %s=%#v -> %q -> %s=%v. Got: %#v", list[0].Name, testValue, converted, list[0].Name, testValue, config.Env[0])
		}
	}
}
