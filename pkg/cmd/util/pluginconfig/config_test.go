package pluginconfig

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	kapiserverinternal "k8s.io/apiserver/pkg/apis/apiserver"
	kapiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"

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

func readAdmissionConfigurationFile(t *testing.T, fileName string, into runtime.Object) *kapiserverinternal.AdmissionConfiguration {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	codec := configapi.Codecs.UniversalDecoder()
	obj, err := runtime.Decode(codec, data)
	if err != nil {
		t.Fatalf("unexpected conversion error: %v", err)
	}
	admissionConfig, ok := obj.(*kapiserverinternal.AdmissionConfiguration)
	if !ok {
		t.Fatalf("expected kapiserverinternal.AdmissionConfiguration, got: %#v", obj)
	}

	if len(admissionConfig.Plugins) != 1 {
		t.Fatalf("expected exactly one plugin config, got: %#v", admissionConfig.Plugins)
	}

	embeddedUnknown, ok := admissionConfig.Plugins[0].Configuration.(*runtime.Unknown)
	if !ok {
		t.Fatalf("expected embedded Unknown, got: %#v", admissionConfig.Plugins[0].Configuration)
	}

	err = runtime.DecodeInto(codec, embeddedUnknown.Raw, into)
	if err != nil {
		t.Fatal(err)
	}

	return admissionConfig
}

func TestGetAdmissionConfigurationConfigWithConfiguration(t *testing.T) {
	configapi.Scheme.AddKnownTypes(oapi.SchemeGroupVersion, &testtypes.TestConfig{})
	configapi.Scheme.AddKnownTypeWithName(latest.Version.WithKind("TestConfig"), &testtypes.TestConfigV1{})
	kapiserverv1alpha1.AddToScheme(configapi.Scheme)
	kapiserverinternal.AddToScheme(configapi.Scheme)

	testConfig := &testtypes.TestConfig{
		Item1: "item1value",
		Item2: []string{"element1", "element2"},
	}

	cfg := configapi.AdmissionPluginConfig{
		Configuration: testConfig,
	}
	fileName, err := GetAdmissionConfigurationConfig("test", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultConfig := &testtypes.TestConfig{}
	admissionConfig := readAdmissionConfigurationFile(t, fileName, resultConfig)

	if !reflect.DeepEqual(testConfig, resultConfig) {
		t.Errorf("Unexpected config. Expected: %#v. Got: %#v", testConfig, admissionConfig.Plugins[0].Configuration)
	}
}

func TestGetAdmissionConfigurationConfigWithLocation(t *testing.T) {
	configapi.Scheme.AddKnownTypes(oapi.SchemeGroupVersion, &testtypes.TestConfig{})
	configapi.Scheme.AddKnownTypeWithName(latest.Version.WithKind("TestConfig"), &testtypes.TestConfigV1{})
	kapiserverv1alpha1.AddToScheme(configapi.Scheme)
	kapiserverinternal.AddToScheme(configapi.Scheme)

	f, err := ioutil.TempFile("", "plugin-config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err = f.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(f.Name())

	testConfig := &testtypes.TestConfig{
		Item1: "item1value",
		Item2: []string{"element1", "element2"},
	}

	testJSON, err := json.Marshal(testConfig)
	if err != nil {
		t.Fatalf("unexpected conversion error: %v", err)
	}
	if err := ioutil.WriteFile(f.Name(), testJSON, 0644); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := configapi.AdmissionPluginConfig{
		Location: f.Name(),
	}
	fileName, err := GetAdmissionConfigurationConfig("test", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultConfig := &testtypes.TestConfig{}
	admissionConfig := readAdmissionConfigurationFile(t, fileName, resultConfig)

	if !reflect.DeepEqual(testConfig, resultConfig) {
		t.Errorf("Unexpected config. Expected: %#v. Got: %#v", testConfig, admissionConfig.Plugins[0].Configuration)
	}
}
