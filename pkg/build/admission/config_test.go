package admission

import (
	"reflect"
	"testing"

	testtypes "github.com/openshift/origin/pkg/build/admission/testing"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapiv1 "github.com/openshift/origin/pkg/cmd/server/api/v1"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestReadPluginConfig(t *testing.T) {
	configapi.Scheme.AddKnownTypes(configapi.SchemeGroupVersion, &testtypes.TestConfig{})
	configapi.Scheme.AddKnownTypeWithName(configapiv1.SchemeGroupVersion.WithKind("TestConfig"), &testtypes.TestConfigV1{})
	configapi.Scheme.AddKnownTypes(configapi.SchemeGroupVersion, &testtypes.OtherTestConfig2{})
	configapi.Scheme.AddKnownTypeWithName(configapiv1.SchemeGroupVersion.WithKind("OtherTestConfig2"), &testtypes.OtherTestConfig2V2{})

	config := &testtypes.TestConfig{}

	expected := &testtypes.TestConfig{
		Item1: "hello",
		Item2: []string{"foo", "bar"},
	}
	pluginCfg := map[string]configapi.AdmissionPluginConfig{"testconfig": {Location: "", Configuration: expected}}
	// The config should match the expected config object
	err := ReadPluginConfig(pluginCfg, "testconfig", config)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !reflect.DeepEqual(config, expected) {
		t.Errorf("config does not equal expected: %#v", config)
	}

	// Passing a nil cfg, should not get an error
	pluginCfg = map[string]configapi.AdmissionPluginConfig{}
	err = ReadPluginConfig(pluginCfg, "testconfig", &testtypes.TestConfig{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Passing the wrong type of destination object should result in an error
	config2 := &testtypes.OtherTestConfig2{}
	pluginCfg = map[string]configapi.AdmissionPluginConfig{"testconfig": {Location: "", Configuration: expected}}
	err = ReadPluginConfig(pluginCfg, "testconfig", config2)
	if err == nil {
		t.Fatalf("expected error")
	}
}
