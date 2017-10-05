package pluginconfig

import (
	"reflect"
	"testing"

	oapi "github.com/openshift/origin/pkg/api"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
	testtypes "github.com/openshift/origin/pkg/cmd/util/pluginconfig/testing"

	// install server api
	_ "github.com/openshift/origin/pkg/cmd/server/api/install"
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
	fileName, err := GetPluginConfig(cfg)
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
