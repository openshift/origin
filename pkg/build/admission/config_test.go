package admission

import (
	"bytes"
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
	config2 := &OtherTestConfig2{}
	err = ReadPluginConfig(bytes.NewBufferString(configString), config2)
	if err == nil {
		t.Fatalf("expected error")
	}
}
