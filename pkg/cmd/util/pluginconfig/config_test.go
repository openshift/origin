package pluginconfig

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/v1"
	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
)

type TestConfig struct {
	unversioned.TypeMeta `json:",inline"`
	Item1                string   `json:"item1"`
	Item2                []string `json:"item2"`
}

func (*TestConfig) IsAnAPIObject() {}

func TestGetPluginConfig(t *testing.T) {
	api.Scheme.AddKnownTypes(oapi.SchemeGroupVersion, &TestConfig{})
	api.Scheme.AddKnownTypes(v1.SchemeGroupVersion, &TestConfig{})

	testConfig := &TestConfig{
		Item1: "item1value",
		Item2: []string{"element1", "element2"},
	}

	cfg := api.AdmissionPluginConfig{
		Location: "/path/to/my/config",
		Configuration: runtime.EmbeddedObject{
			Object: testConfig,
		},
	}
	fileName, err := GetPluginConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resultConfig := &TestConfig{}
	if err = latest.ReadYAMLFile(fileName, resultConfig); err != nil {
		t.Fatalf("error reading config file: %v", err)
	}
	if !reflect.DeepEqual(testConfig, resultConfig) {
		t.Errorf("Unexpected config. Expected: %#v. Got: %#v", testConfig, resultConfig)
	}
}
