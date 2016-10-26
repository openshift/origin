package admission

import (
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api/unversioned"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapiv1 "github.com/openshift/origin/pkg/cmd/server/api/v1"

	_ "github.com/openshift/origin/pkg/api/install"
)

type TestConfig struct {
	unversioned.TypeMeta

	Item1 string   `json:"item1"`
	Item2 []string `json:"item2"`
}

type TestConfigV1 struct {
	unversioned.TypeMeta

	Item1 string   `json:"item1"`
	Item2 []string `json:"item2"`
}

type OtherTestConfig2 struct {
	unversioned.TypeMeta
	Thing string `json:"thing"`
}

type OtherTestConfig2V2 struct {
	unversioned.TypeMeta
	Thing string `json:"thing"`
}

func (obj *TestConfig) GetObjectKind() unversioned.ObjectKind         { return &obj.TypeMeta }
func (obj *TestConfigV1) GetObjectKind() unversioned.ObjectKind       { return &obj.TypeMeta }
func (obj *OtherTestConfig2) GetObjectKind() unversioned.ObjectKind   { return &obj.TypeMeta }
func (obj *OtherTestConfig2V2) GetObjectKind() unversioned.ObjectKind { return &obj.TypeMeta }

func TestReadPluginConfig(t *testing.T) {
	configapi.Scheme.AddKnownTypes(configapi.SchemeGroupVersion, &TestConfig{})
	configapi.Scheme.AddKnownTypeWithName(configapiv1.SchemeGroupVersion.WithKind("TestConfig"), &TestConfigV1{})
	configapi.Scheme.AddKnownTypes(configapi.SchemeGroupVersion, &OtherTestConfig2{})
	configapi.Scheme.AddKnownTypeWithName(configapiv1.SchemeGroupVersion.WithKind("OtherTestConfig2"), &OtherTestConfig2V2{})

	config := &TestConfig{}

	expected := &TestConfig{
		Item1: "hello",
		Item2: []string{"foo", "bar"},
	}
	pluginCfg := map[string]configapi.AdmissionPluginConfig{"testconfig": {"", expected}}
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
	err = ReadPluginConfig(pluginCfg, "testconfig", &TestConfig{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Passing the wrong type of destination object should result in an error
	config2 := &OtherTestConfig2{}
	pluginCfg = map[string]configapi.AdmissionPluginConfig{"testconfig": {"", expected}}
	err = ReadPluginConfig(pluginCfg, "testconfig", config2)
	if err == nil {
		t.Fatalf("expected error")
	}
}
