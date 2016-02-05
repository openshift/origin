package admission

import (
	"bytes"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api/unversioned"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

type TestConfig struct {
	unversioned.TypeMeta

	Item1 string   `json:"item1"`
	Item2 []string `json:"item2"`
}

func (*TestConfig) IsAnAPIObject() {}

type TestConfig2 struct {
	unversioned.TypeMeta
	Item1 string `json:"item1"`
}

func (*TestConfig2) IsAnAPIObject() {}

func TestReadPluginConfig(t *testing.T) {
	groupVersion := unversioned.GroupVersion{Group: "", Version: ""}
	v1GroupVersion := unversioned.GroupVersion{Group: "", Version: "v1"}
	configapi.Scheme.AddKnownTypes(groupVersion, &TestConfig{})
	configapi.Scheme.AddKnownTypes(v1GroupVersion, &TestConfig{})
	configapi.Scheme.AddKnownTypes(groupVersion, &TestConfig2{})
	configapi.Scheme.AddKnownTypes(v1GroupVersion, &TestConfig2{})

	configString := `apiVersion: v1
kind: TestConfig
item1: hello
item2:
- foo
- bar
`
	config := &TestConfig{}

	expected := &TestConfig{
		Item1: "hello",
		Item2: []string{"foo", "bar"},
	}
	// The config should match the expected config object
	err := ReadPluginConfig(bytes.NewBufferString(configString), config)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !reflect.DeepEqual(config, expected) {
		t.Errorf("config does not equal expected: %#v", config)
	}

	// Passing a nil reader, should not get an error
	var nilBuffer *bytes.Buffer
	err = ReadPluginConfig(nilBuffer, &TestConfig{})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Passing the wrong type of destination object should result in an error
	config2 := &TestConfig2{}
	err = ReadPluginConfig(bytes.NewBufferString(configString), config2)
	if err == nil {
		t.Fatalf("expected error")
	}
}
