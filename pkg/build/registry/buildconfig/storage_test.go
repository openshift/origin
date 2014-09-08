package buildconfig

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/v1beta1"
	"github.com/openshift/origin/pkg/build/registry/test"
)

func TestNewConfig(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := Storage{registry: &mockRegistry}
	obj := storage.New()
	_, ok := obj.(*api.BuildConfig)
	if !ok {
		t.Error("New did not return an object of type *BuildConfig")
	}
}

func TestGetConfig(t *testing.T) {
	expectedConfig := mockBuildConfig()
	mockRegistry := test.BuildConfigRegistry{BuildConfig: expectedConfig}
	storage := Storage{registry: &mockRegistry}
	configObj, err := storage.Get("foo")
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	config, ok := configObj.(*api.BuildConfig)
	if !ok {
		t.Errorf("A build config was not returned: %v", configObj)
	}
	if config.ID != expectedConfig.ID {
		t.Errorf("Unexpected build config returned: %v", config)
	}
}

func TestGetConfigError(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{Err: fmt.Errorf("get error")}
	storage := Storage{registry: &mockRegistry}
	buildObj, err := storage.Get("foo")
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}
	if buildObj != nil {
		t.Errorf("Unexpected non-nil build: %#v", buildObj)
	}
}

func TestDeleteBuild(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	configId := "test-config-id"
	storage := Storage{registry: &mockRegistry}
	channel, err := storage.Delete(configId)
	if err != nil {
		t.Errorf("Unexpected error when deleting: %v", err)
	}
	select {
	case result := <-channel:
		status, ok := result.(*kubeapi.Status)
		if !ok {
			t.Errorf("Unexpected operation result: %v", result)
		}
		if status.Status != kubeapi.StatusSuccess {
			t.Errorf("Unexpected failure status: %v", status)
		}
		if mockRegistry.DeletedConfigId != configId {
			t.Errorf("Unexpected build id was deleted: %v", mockRegistry.DeletedConfigId)
		}
		// expected case
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestDeleteBuildError(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{Err: fmt.Errorf("Delete error")}
	configId := "test-config-id"
	storage := Storage{registry: &mockRegistry}
	channel, _ := storage.Delete(configId)
	select {
	case result := <-channel:
		status, ok := result.(*kubeapi.Status)
		if !ok {
			t.Errorf("Unexpected operation result: %#v", channel)
		}
		if status.Message != mockRegistry.Err.Error() {
			t.Errorf("Unexpected status returned: %#v", status)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestListConfigsError(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{
		Err: fmt.Errorf("test error"),
	}
	storage := Storage{
		registry: &mockRegistry,
	}
	configs, err := storage.List(nil)
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}
	if configs != nil {
		t.Errorf("Unexpected non-nil buildConfig list: %#v", configs)
	}
}

func TestListEmptyConfigList(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{BuildConfigs: &api.BuildConfigList{JSONBase: kubeapi.JSONBase{ResourceVersion: 1}}}
	storage := Storage{
		registry: &mockRegistry,
	}
	buildConfigs, err := storage.List(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(buildConfigs.(*api.BuildConfigList).Items) != 0 {
		t.Errorf("Unexpected non-zero ctrl list: %#v", buildConfigs)
	}
	if buildConfigs.(*api.BuildConfigList).ResourceVersion != 1 {
		t.Errorf("Unexpected resource version: %#v", buildConfigs)
	}
}

func TestListConfigs(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{
		BuildConfigs: &api.BuildConfigList{
			Items: []api.BuildConfig{
				{
					JSONBase: kubeapi.JSONBase{
						ID: "foo",
					},
				},
				{
					JSONBase: kubeapi.JSONBase{
						ID: "bar",
					},
				},
			},
		},
	}
	storage := Storage{
		registry: &mockRegistry,
	}
	configsObj, err := storage.List(labels.Everything())
	configs := configsObj.(*api.BuildConfigList)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(configs.Items) != 2 {
		t.Errorf("Unexpected buildConfig list: %#v", configs)
	}
	if configs.Items[0].ID != "foo" {
		t.Errorf("Unexpected buildConfig: %#v", configs.Items[0])
	}
	if configs.Items[1].ID != "bar" {
		t.Errorf("Unexpected buildConfig: %#v", configs.Items[1])
	}
}

func TestBuildConfigDecode(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := Storage{
		registry: &mockRegistry,
	}
	buildConfig := &api.BuildConfig{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
	}
	body, err := runtime.Encode(buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	buildConfigOut := storage.New()
	if err := runtime.DecodeInto(body, buildConfigOut); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(buildConfig, buildConfigOut) {
		t.Errorf("Expected %#v, found %#v", buildConfig, buildConfigOut)
	}
}

func TestBuildConfigParsing(t *testing.T) {
	expectedBuildConfig := mockBuildConfig()
	file, err := ioutil.TempFile("", "buildConfig")
	fileName := file.Name()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	data, err := json.Marshal(expectedBuildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = file.Write(data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = file.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	data, err = ioutil.ReadFile(fileName)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	var buildConfig api.BuildConfig
	err = json.Unmarshal(data, &buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(&buildConfig, expectedBuildConfig) {
		t.Errorf("Parsing failed: %s\ngot: %#v\nexpected: %#v", string(data), &buildConfig, expectedBuildConfig)
	}
}

func TestCreateBuildConfig(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := Storage{&mockRegistry}
	buildConfig := mockBuildConfig()
	channel, err := storage.Create(buildConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case <-channel:
		// expected case
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func mockBuildConfig() *api.BuildConfig {
	return &api.BuildConfig{
		JSONBase: kubeapi.JSONBase{
			ID: "dataBuild",
		},
		DesiredInput: api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "http://my.build.com/the/buildConfig/Dockerfile",
			ImageTag:  "repository/dataBuild",
		},
		Labels: map[string]string{
			"name": "dataBuild",
		},
	}
}

func TestUpdateBuildConfig(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := Storage{&mockRegistry}
	buildConfig := mockBuildConfig()
	channel, err := storage.Update(buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	select {
	case result := <-channel:
		switch obj := result.(type) {
		case *kubeapi.Status:
			t.Errorf("Unexpected operation error: %v", obj)

		case *api.BuildConfig:
			if !reflect.DeepEqual(buildConfig, obj) {
				t.Errorf("Updated build does not match input build."+
					" Expected: %v, Got: %v", buildConfig, obj)
			}
		default:
			t.Errorf("Unexpected result type: %v", result)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestUpdateBuildConfigError(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{Err: fmt.Errorf("Update error")}
	storage := Storage{&mockRegistry}
	buildConfig := mockBuildConfig()
	channel, err := storage.Update(buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	select {
	case result := <-channel:
		switch obj := result.(type) {
		case *kubeapi.Status:
			if obj.Message != mockRegistry.Err.Error() {
				t.Errorf("Unexpected error result: %v", obj)
			}
		default:
			t.Errorf("Unexpected result type: %v", result)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestBuildConfigStorageValidatesCreate(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := Storage{&mockRegistry}
	failureCases := map[string]api.BuildConfig{
		"blank sourceURI": {
			JSONBase: kubeapi.JSONBase{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI:    "",
				ImageTag:     "data/image",
				Type:         api.STIBuildType,
				BuilderImage: "builder/image",
			},
		},
		"blank ImageTag": {
			JSONBase: kubeapi.JSONBase{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI: "http://github.com/test/source",
				ImageTag:  "",
				Type:      api.DockerBuildType,
			},
		},
		"blank BuilderImage": {
			JSONBase: kubeapi.JSONBase{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI:    "http://github.com/test/source",
				ImageTag:     "data/image",
				Type:         api.STIBuildType,
				BuilderImage: "",
			},
		},
	}
	for desc, failureCase := range failureCases {
		c, err := storage.Create(&failureCase)
		if c != nil {
			t.Errorf("%s: Expected nil channel", desc)
		}
		if !errors.IsInvalid(err) {
			t.Errorf("%s: Expected to get an invalid resource error, got %v", desc, err)
		}
	}
}

func TestBuildStorageValidatesUpdate(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := Storage{&mockRegistry}
	failureCases := map[string]api.BuildConfig{
		"empty ID": {
			JSONBase: kubeapi.JSONBase{ID: ""},
			DesiredInput: api.BuildInput{
				SourceURI: "http://github.com/test/source",
				ImageTag:  "data/image",
				Type:      api.DockerBuildType,
			},
		},
		"blank sourceURI": {
			JSONBase: kubeapi.JSONBase{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI:    "",
				ImageTag:     "data/image",
				Type:         api.STIBuildType,
				BuilderImage: "builder/image",
			},
		},
		"blank ImageTag": {
			JSONBase: kubeapi.JSONBase{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI: "http://github.com/test/source",
				ImageTag:  "",
				Type:      api.DockerBuildType,
			},
		},
		"blank BuilderImage on STIBuildType": {
			JSONBase: kubeapi.JSONBase{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI:    "http://github.com/test/source",
				ImageTag:     "data/image",
				Type:         api.STIBuildType,
				BuilderImage: "",
			},
		},
		"non-blank BuilderImage on DockerBuildType": {
			JSONBase: kubeapi.JSONBase{ID: "abc"},
			DesiredInput: api.BuildInput{
				SourceURI:    "http://github.com/test/source",
				ImageTag:     "data/image",
				Type:         api.DockerBuildType,
				BuilderImage: "builder/image",
			},
		},
	}
	for desc, failureCase := range failureCases {
		c, err := storage.Update(&failureCase)
		if c != nil {
			t.Errorf("%s: Expected nil channel", desc)
		}
		if !errors.IsInvalid(err) {
			t.Errorf("%s: Expected to get an invalid resource error, got %v", desc, err)
		}
	}
}
