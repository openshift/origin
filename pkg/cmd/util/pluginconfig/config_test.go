package pluginconfig

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api/unversioned"

	oapi "github.com/openshift/origin/pkg/api"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"

	_ "github.com/openshift/origin/pkg/cmd/server/api/install"
)

type TestConfig struct {
	unversioned.TypeMeta `json:",inline"`
	Item1                string   `json:"item1"`
	Item2                []string `json:"item2"`
}

func (obj *TestConfig) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }

type TestConfigV1 struct {
	unversioned.TypeMeta `json:",inline"`
	Item1                string   `json:"item1"`
	Item2                []string `json:"item2"`
}

func (obj *TestConfigV1) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }

func TestGetPluginConfig(t *testing.T) {
	configapi.Scheme.AddKnownTypes(oapi.SchemeGroupVersion, &TestConfig{})
	configapi.Scheme.AddKnownTypeWithName(latest.Version.WithKind("TestConfig"), &TestConfigV1{})

	testConfig := &TestConfig{
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
	resultConfig := &TestConfig{}
	if err = latest.ReadYAMLFileInto(fileName, resultConfig); err != nil {
		t.Fatalf("error reading config file: %v", err)
	}
	if !reflect.DeepEqual(testConfig, resultConfig) {
		t.Errorf("Unexpected config. Expected: %#v. Got: %#v", testConfig, resultConfig)
	}
}
