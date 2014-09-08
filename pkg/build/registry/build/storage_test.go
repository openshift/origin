package build

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

func TestNewBuild(t *testing.T) {
	mockRegistry := test.BuildRegistry{}
	storage := Storage{registry: &mockRegistry}
	obj := storage.New()
	_, ok := obj.(*api.Build)
	if !ok {
		t.Errorf("New did not return an object of type *Build")
	}
}

func TestGetBuild(t *testing.T) {
	expectedBuild := mockBuild()
	mockRegistry := test.BuildRegistry{Build: expectedBuild}
	storage := Storage{registry: &mockRegistry}
	buildObj, err := storage.Get("foo")
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	build, ok := buildObj.(*api.Build)
	if !ok {
		t.Errorf("A build was not returned: %v", buildObj)
	}
	if build.ID != expectedBuild.ID {
		t.Errorf("Unexpected build returned: %v", build)
	}
}

func TestGetBuildError(t *testing.T) {
	mockRegistry := test.BuildRegistry{Err: fmt.Errorf("get error")}
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
	mockRegistry := test.BuildRegistry{}
	buildId := "test-build-id"
	storage := Storage{registry: &mockRegistry}
	channel, err := storage.Delete(buildId)
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
		if mockRegistry.DeletedBuildId != buildId {
			t.Errorf("Unexpected build id was deleted: %v", mockRegistry.DeletedBuildId)
		}
		// expected case
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestDeleteBuildError(t *testing.T) {
	mockRegistry := test.BuildRegistry{Err: fmt.Errorf("Delete error")}
	buildId := "test-build-id"
	storage := Storage{registry: &mockRegistry}
	channel, _ := storage.Delete(buildId)
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

func TestListBuildsError(t *testing.T) {
	mockRegistry := test.BuildRegistry{
		Err: fmt.Errorf("test error"),
	}
	storage := Storage{
		registry: &mockRegistry,
	}
	builds, err := storage.List(nil)
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}
	if builds != nil {
		t.Errorf("Unexpected non-nil build list: %#v", builds)
	}
}

func TestListEmptyBuildList(t *testing.T) {
	mockRegistry := test.BuildRegistry{Builds: &api.BuildList{JSONBase: kubeapi.JSONBase{ResourceVersion: 1}}}
	storage := Storage{
		registry: &mockRegistry,
	}
	builds, err := storage.List(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(builds.(*api.BuildList).Items) != 0 {
		t.Errorf("Unexpected non-zero ctrl list: %#v", builds)
	}
	if builds.(*api.BuildList).ResourceVersion != 1 {
		t.Errorf("Unexpected resource version: %#v", builds)
	}
}

func TestListBuilds(t *testing.T) {
	mockRegistry := test.BuildRegistry{
		Builds: &api.BuildList{
			Items: []api.Build{
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
	buildsObj, err := storage.List(labels.Everything())
	builds := buildsObj.(*api.BuildList)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(builds.Items) != 2 {
		t.Errorf("Unexpected build list: %#v", builds)
	}
	if builds.Items[0].ID != "foo" {
		t.Errorf("Unexpected build: %#v", builds.Items[0])
	}
	if builds.Items[1].ID != "bar" {
		t.Errorf("Unexpected build: %#v", builds.Items[1])
	}
}

func TestBuildDecode(t *testing.T) {
	mockRegistry := test.BuildRegistry{}
	storage := Storage{
		registry: &mockRegistry,
	}
	build := &api.Build{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
	}
	body, err := runtime.Encode(build)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	buildOut := storage.New()
	if err := runtime.DecodeInto(body, buildOut); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(build, buildOut) {
		t.Errorf("Expected %#v, found %#v", build, buildOut)
	}
}

func TestBuildParsing(t *testing.T) {
	expectedBuild := mockBuild()
	file, err := ioutil.TempFile("", "build")
	fileName := file.Name()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	data, err := json.Marshal(expectedBuild)
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

	var build api.Build
	err = json.Unmarshal(data, &build)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(build, *expectedBuild) {
		t.Errorf("Parsing failed: %s\ngot: %#v\nexpected: %#v", string(data), build, *expectedBuild)
	}
}

func TestCreateBuild(t *testing.T) {
	mockRegistry := test.BuildRegistry{}
	storage := Storage{&mockRegistry}
	build := mockBuild()
	channel, err := storage.Create(build)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	select {
	case result := <-channel:
		switch obj := result.(type) {
		case *kubeapi.Status:
			t.Errorf("Unexpected operation error: %v", obj)

		case *api.Build:
			if !reflect.DeepEqual(build, obj) {
				t.Errorf("Created build does not match input build."+
					" Expected: %v, Got: %v", build, obj)
			}
		default:
			t.Errorf("Unexpected result type: %v", result)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestUpdateBuild(t *testing.T) {
	mockRegistry := test.BuildRegistry{}
	storage := Storage{&mockRegistry}
	build := mockBuild()
	channel, err := storage.Update(build)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	select {
	case result := <-channel:
		switch obj := result.(type) {
		case *kubeapi.Status:
			t.Errorf("Unexpected operation error: %v", obj)

		case *api.Build:
			if !reflect.DeepEqual(build, obj) {
				t.Errorf("Updated build does not match input build."+
					" Expected: %v, Got: %v", build, obj)
			}
		default:
			t.Errorf("Unexpected result type: %v", result)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestUpdateBuildError(t *testing.T) {
	mockRegistry := test.BuildRegistry{Err: fmt.Errorf("Update error")}
	storage := Storage{&mockRegistry}
	build := mockBuild()
	channel, err := storage.Update(build)
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

func TestBuildStorageValidatesCreate(t *testing.T) {
	mockRegistry := test.BuildRegistry{}
	storage := Storage{&mockRegistry}
	failureCases := map[string]api.Build{
		"empty input": {
			JSONBase: kubeapi.JSONBase{ID: "abc"},
			Input:    api.BuildInput{},
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
	mockRegistry := test.BuildRegistry{}
	storage := Storage{&mockRegistry}
	failureCases := map[string]api.Build{
		"empty ID": {
			JSONBase: kubeapi.JSONBase{ID: ""},
			Input: api.BuildInput{
				Type:      api.DockerBuildType,
				SourceURI: "http://my.build.com/the/build/Dockerfile",
				ImageTag:  "repository/dataBuild",
			},
		},
		"empty build input": {
			JSONBase: kubeapi.JSONBase{ID: "abc"},
			Input:    api.BuildInput{},
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

func mockBuild() *api.Build {
	return &api.Build{
		JSONBase: kubeapi.JSONBase{
			ID: "dataBuild",
		},
		Input: api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "http://my.build.com/the/build/Dockerfile",
			ImageTag:  "repository/dataBuild",
		},
		Status: api.BuildPending,
		PodID:  "-the-pod-id",
		Labels: map[string]string{
			"name": "dataBuild",
		},
	}
}
