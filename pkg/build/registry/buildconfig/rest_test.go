package buildconfig

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
)

func TestNewConfig(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{&mockRegistry}
	obj := storage.New()
	_, ok := obj.(*api.BuildConfig)
	if !ok {
		t.Error("New did not return an object of type *BuildConfig")
	}
}

func TestGetConfig(t *testing.T) {
	expectedConfig := mockBuildConfig()
	mockRegistry := test.BuildConfigRegistry{BuildConfig: expectedConfig}
	storage := REST{&mockRegistry}
	configObj, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	config, ok := configObj.(*api.BuildConfig)
	if !ok {
		t.Errorf("A build config was not returned: %v", configObj)
	}
	if config.Name != expectedConfig.Name {
		t.Errorf("Unexpected build config returned: %v", config)
	}
}

func TestGetConfigError(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{Err: fmt.Errorf("get error")}
	storage := REST{&mockRegistry}
	buildObj, err := storage.Get(kapi.NewDefaultContext(), "foo")
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
	storage := REST{&mockRegistry}
	channel, err := storage.Delete(kapi.NewDefaultContext(), configId)
	if err != nil {
		t.Errorf("Unexpected error when deleting: %v", err)
	}
	select {
	case result := <-channel:
		status, ok := result.Object.(*kapi.Status)
		if !ok {
			t.Errorf("Unexpected operation result: %v", result)
		}
		if status.Status != kapi.StatusSuccess {
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
	storage := REST{&mockRegistry}
	channel, _ := storage.Delete(kapi.NewDefaultContext(), configId)
	select {
	case result := <-channel:
		status, ok := result.Object.(*kapi.Status)
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
	storage := REST{&mockRegistry}
	configs, err := storage.List(kapi.NewDefaultContext(), nil, nil)
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}
	if configs != nil {
		t.Errorf("Unexpected non-nil buildConfig list: %#v", configs)
	}
}

func TestListEmptyConfigList(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{BuildConfigs: &api.BuildConfigList{ListMeta: kapi.ListMeta{ResourceVersion: "1"}}}
	storage := REST{&mockRegistry}
	buildConfigs, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(buildConfigs.(*api.BuildConfigList).Items) != 0 {
		t.Errorf("Unexpected non-zero ctrl list: %#v", buildConfigs)
	}
	if buildConfigs.(*api.BuildConfigList).ResourceVersion != "1" {
		t.Errorf("Unexpected resource version: %#v", buildConfigs)
	}
}

func TestListConfigs(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{
		BuildConfigs: &api.BuildConfigList{
			Items: []api.BuildConfig{
				{
					ObjectMeta: kapi.ObjectMeta{
						Name: "foo",
					},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Name: "bar",
					},
				},
			},
		},
	}
	storage := REST{&mockRegistry}
	configsObj, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), labels.Everything())
	configs := configsObj.(*api.BuildConfigList)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(configs.Items) != 2 {
		t.Errorf("Unexpected buildConfig list: %#v", configs)
	}
	if configs.Items[0].Name != "foo" {
		t.Errorf("Unexpected buildConfig: %#v", configs.Items[0])
	}
	if configs.Items[1].Name != "bar" {
		t.Errorf("Unexpected buildConfig: %#v", configs.Items[1])
	}
}

func TestBuildConfigDecode(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{registry: &mockRegistry}
	buildConfig := &api.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	}
	body, err := latest.Codec.Encode(buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	buildConfigOut := storage.New()
	if err := latest.Codec.DecodeInto(body, buildConfigOut); err != nil {
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
	storage := REST{&mockRegistry}
	buildConfig := mockBuildConfig()
	channel, err := storage.Create(kapi.NewDefaultContext(), buildConfig)
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
		ObjectMeta: kapi.ObjectMeta{
			Name:      "dataBuild",
			Namespace: kapi.NamespaceDefault,
			Labels: map[string]string{
				"name": "dataBuild",
			},
		},
		Parameters: api.BuildParameters{
			Source: api.BuildSource{
				Type: api.BuildSourceGit,
				Git: &api.GitBuildSource{
					URI: "http://my.build.com/the/build/Dockerfile",
				},
			},
			Strategy: api.BuildStrategy{
				Type: api.STIBuildStrategyType,
				STIStrategy: &api.STIBuildStrategy{
					BuilderImage: "builder/image",
				},
			},
			Output: api.BuildOutput{
				ImageTag: "repository/dataBuild",
			},
		},
	}
}

func TestUpdateBuildConfig(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{&mockRegistry}
	buildConfig := mockBuildConfig()
	channel, err := storage.Update(kapi.NewDefaultContext(), buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	select {
	case result := <-channel:
		switch obj := result.Object.(type) {
		case *kapi.Status:
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
	storage := REST{&mockRegistry}
	buildConfig := mockBuildConfig()
	channel, err := storage.Update(kapi.NewDefaultContext(), buildConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	select {
	case result := <-channel:
		switch obj := result.Object.(type) {
		case *kapi.Status:
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

func TestBuildConfigRESTValidatesCreate(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{&mockRegistry}
	failureCases := map[string]api.BuildConfig{
		"blank sourceURI": {
			ObjectMeta: kapi.ObjectMeta{Name: "abc"},
			Parameters: api.BuildParameters{
				Source: api.BuildSource{
					Type: api.BuildSourceGit,
					Git: &api.GitBuildSource{
						URI: "",
					},
				},
				Strategy: api.BuildStrategy{
					Type: api.STIBuildStrategyType,
					STIStrategy: &api.STIBuildStrategy{
						BuilderImage: "builder/image",
					},
				},
				Output: api.BuildOutput{
					ImageTag: "data/image",
				},
			},
		},
		"blank ImageTag": {
			ObjectMeta: kapi.ObjectMeta{Name: "abc"},
			Parameters: api.BuildParameters{
				Source: api.BuildSource{
					Type: api.BuildSourceGit,
					Git: &api.GitBuildSource{
						URI: "http://github.com/test/source",
					},
				},
				Output: api.BuildOutput{
					ImageTag: "",
				},
			},
		},
		"blank BuilderImage": {
			ObjectMeta: kapi.ObjectMeta{Name: "abc"},
			Parameters: api.BuildParameters{
				Source: api.BuildSource{
					Type: api.BuildSourceGit,
					Git: &api.GitBuildSource{
						URI: "http://github.com/test/source",
					},
				},
				Strategy: api.BuildStrategy{
					Type: api.STIBuildStrategyType,
					STIStrategy: &api.STIBuildStrategy{
						BuilderImage: "",
					},
				},
				Output: api.BuildOutput{
					ImageTag: "data/image",
				},
			},
		},
	}
	for desc, failureCase := range failureCases {
		c, err := storage.Create(kapi.NewDefaultContext(), &failureCase)
		if c != nil {
			t.Errorf("%s: Expected nil channel", desc)
		}
		if !errors.IsInvalid(err) {
			t.Errorf("%s: Expected to get an invalid resource error, got %v", desc, err)
		}
	}
}

func TestBuildRESTValidatesUpdate(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{&mockRegistry}
	failureCases := map[string]api.BuildConfig{
		"empty ID": {
			ObjectMeta: kapi.ObjectMeta{Name: ""},
			Parameters: api.BuildParameters{
				Source: api.BuildSource{
					Type: api.BuildSourceGit,
					Git: &api.GitBuildSource{
						URI: "http://github.com/test/source",
					},
				},
				Output: api.BuildOutput{
					ImageTag: "data/image",
				},
			},
		},
		"blank sourceURI": {
			ObjectMeta: kapi.ObjectMeta{Name: "abc"},
			Parameters: api.BuildParameters{
				Source: api.BuildSource{
					Type: api.BuildSourceGit,
					Git: &api.GitBuildSource{
						URI: "",
					},
				},
				Strategy: api.BuildStrategy{
					Type: api.STIBuildStrategyType,
					STIStrategy: &api.STIBuildStrategy{
						BuilderImage: "builder/image",
					},
				},
				Output: api.BuildOutput{
					ImageTag: "data/image",
				},
			},
		},
		"blank ImageTag": {
			ObjectMeta: kapi.ObjectMeta{Name: "abc"},
			Parameters: api.BuildParameters{
				Source: api.BuildSource{
					Type: api.BuildSourceGit,
					Git: &api.GitBuildSource{
						URI: "http://github.com/test/source",
					},
				},
				Output: api.BuildOutput{
					ImageTag: "",
				},
			},
		},
		"blank BuilderImage on STIBuildType": {
			ObjectMeta: kapi.ObjectMeta{Name: "abc"},
			Parameters: api.BuildParameters{
				Source: api.BuildSource{
					Type: api.BuildSourceGit,
					Git: &api.GitBuildSource{
						URI: "http://github.com/test/source",
					},
				},
				Strategy: api.BuildStrategy{
					Type: api.STIBuildStrategyType,
					STIStrategy: &api.STIBuildStrategy{
						BuilderImage: "",
					},
				},
				Output: api.BuildOutput{
					ImageTag: "data/image",
				},
			},
		},
	}
	for desc, failureCase := range failureCases {
		c, err := storage.Update(kapi.NewDefaultContext(), &failureCase)
		if c != nil {
			t.Errorf("%s: Expected nil channel", desc)
		}
		if !errors.IsInvalid(err) {
			t.Errorf("%s: Expected to get an invalid resource error, got %v", desc, err)
		}
	}
}

func TestCreateBuildConfigConflictingNamespace(t *testing.T) {
	storage := REST{}

	channel, err := storage.Create(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "some-value"},
	})

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func TestUpdateBuildConfigConflictingNamespace(t *testing.T) {
	mockRegistry := test.BuildConfigRegistry{}
	storage := REST{&mockRegistry}

	buildConfig := mockBuildConfig()
	channel, err := storage.Update(kapi.WithNamespace(kapi.NewContext(), "legal-name"), buildConfig)

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func checkExpectedNamespaceError(t *testing.T, err error) {
	expectedError := "BuildConfig.Namespace does not match the provided context"
	if err == nil {
		t.Errorf("Expected '" + expectedError + "', but we didn't get one")
	} else {
		e, ok := err.(kclient.APIStatus)
		if !ok {
			t.Errorf("error was not a statusError: %v", err)
		}
		if e.Status().Code != http.StatusConflict {
			t.Errorf("Unexpected failure status: %v", e.Status())
		}
		if strings.Index(err.Error(), expectedError) == -1 {
			t.Errorf("Expected '"+expectedError+"' error, got '%v'", err.Error())
		}
	}

}
