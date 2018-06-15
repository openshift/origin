package pluginconfig

import (
	"reflect"
	"testing"

	oapi "github.com/openshift/origin/pkg/api"
	testtypes "github.com/openshift/origin/pkg/build/controller/build/pluginconfig/testing"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	configapiv1 "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"

	// install server api
	_ "github.com/openshift/origin/pkg/api/install"
	_ "github.com/openshift/origin/pkg/cmd/server/apis/config/install"
)

func TestGetPluginConfig(t *testing.T) {
	configapi.Scheme.AddKnownTypes(oapi.SchemeGroupVersion, &testtypes.TestConfig{})
	configapi.Scheme.AddKnownTypeWithName(latest.Version.WithKind("TestConfig"), &testtypes.TestConfigV1{})

	testConfig := &testtypes.TestConfig{
		Item1: "item1value",
		Item2: []string{"element1", "element2"},
	}

	cfg := configapi.AdmissionPluginConfig{
		Location:      "/path/to/my/config",
		Configuration: testConfig,
	}
	fileName, err := getPluginConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resultConfig := &testtypes.TestConfig{}
	if err = latest.ReadYAMLFileInto(fileName, resultConfig); err != nil {
		t.Fatalf("error reading config file: %v", err)
	}
	if !reflect.DeepEqual(testConfig, resultConfig) {
		t.Errorf("Unexpected config. Expected: %#v. Got: %#v", testConfig, resultConfig)
	}
}

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
	pluginCfg := map[string]*configapi.AdmissionPluginConfig{"testconfig": {Location: "", Configuration: expected}}
	// The config should match the expected config object
	err := ReadPluginConfig(pluginCfg, "testconfig", config)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !reflect.DeepEqual(config, expected) {
		t.Errorf("config does not equal expected: %#v", config)
	}

	// Passing a nil cfg, should not get an error
	pluginCfg = map[string]*configapi.AdmissionPluginConfig{}
	err = ReadPluginConfig(pluginCfg, "testconfig", &testtypes.TestConfig{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Passing the wrong type of destination object should result in an error
	config2 := &testtypes.OtherTestConfig2{}
	pluginCfg = map[string]*configapi.AdmissionPluginConfig{"testconfig": {Location: "", Configuration: expected}}
	err = ReadPluginConfig(pluginCfg, "testconfig", config2)
	if err == nil {
		t.Fatalf("expected error")
	}
}
