package build

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
	_ "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/test"
)

func TestNewBuild(t *testing.T) {
	mockRegistry := test.BuildRegistry{}
	storage := REST{&mockRegistry}
	obj := storage.New()
	_, ok := obj.(*api.Build)
	if !ok {
		t.Errorf("New did not return an object of type *Build")
	}
}

func TestGetBuild(t *testing.T) {
	expectedBuild := mockBuild()
	mockRegistry := test.BuildRegistry{Build: expectedBuild}
	storage := REST{&mockRegistry}
	buildObj, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	build, ok := buildObj.(*api.Build)
	if !ok {
		t.Errorf("A build was not returned: %v", buildObj)
	}
	if build.Name != expectedBuild.Name {
		t.Errorf("Unexpected build returned: %v", build)
	}
}

func TestGetBuildError(t *testing.T) {
	mockRegistry := test.BuildRegistry{Err: fmt.Errorf("get error")}
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
	mockRegistry := test.BuildRegistry{}
	buildId := "test-build-id"
	storage := REST{&mockRegistry}
	channel, err := storage.Delete(kapi.NewDefaultContext(), buildId)
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
	storage := REST{&mockRegistry}
	channel, _ := storage.Delete(kapi.NewDefaultContext(), buildId)
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

func TestListBuildsError(t *testing.T) {
	mockRegistry := test.BuildRegistry{
		Err: fmt.Errorf("test error"),
	}
	storage := REST{&mockRegistry}
	builds, err := storage.List(kapi.NewDefaultContext(), nil, nil)
	if err != mockRegistry.Err {
		t.Errorf("Expected %#v, Got %#v", mockRegistry.Err, err)
	}
	if builds != nil {
		t.Errorf("Unexpected non-nil build list: %#v", builds)
	}
}

func TestListEmptyBuildList(t *testing.T) {
	mockRegistry := test.BuildRegistry{Builds: &api.BuildList{ListMeta: kapi.ListMeta{ResourceVersion: "1"}}}
	storage := REST{&mockRegistry}
	builds, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(builds.(*api.BuildList).Items) != 0 {
		t.Errorf("Unexpected non-zero ctrl list: %#v", builds)
	}
	if builds.(*api.BuildList).ResourceVersion != "1" {
		t.Errorf("Unexpected resource version: %#v", builds)
	}
}

func TestListBuilds(t *testing.T) {
	mockRegistry := test.BuildRegistry{
		Builds: &api.BuildList{
			Items: []api.Build{
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
	storage := REST{registry: &mockRegistry}
	buildsObj, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), labels.Everything())
	builds := buildsObj.(*api.BuildList)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(builds.Items) != 2 {
		t.Errorf("Unexpected build list: %#v", builds)
	}
	if builds.Items[0].Name != "foo" {
		t.Errorf("Unexpected build: %#v", builds.Items[0])
	}
	if builds.Items[1].Name != "bar" {
		t.Errorf("Unexpected build: %#v", builds.Items[1])
	}
}

func TestBuildDecode(t *testing.T) {
	mockRegistry := test.BuildRegistry{}
	storage := REST{&mockRegistry}
	build := &api.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	}
	body, err := latest.Codec.Encode(build)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	buildOut := storage.New()
	if err := latest.Codec.DecodeInto(body, buildOut); err != nil {
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
	storage := REST{&mockRegistry}
	build := mockBuild()
	channel, err := storage.Create(kapi.NewDefaultContext(), build)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	select {
	case result := <-channel:
		switch obj := result.Object.(type) {
		case *kapi.Status:
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
	storage := REST{&mockRegistry}
	build := mockBuild()
	channel, err := storage.Update(kapi.NewDefaultContext(), build)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	select {
	case result := <-channel:
		switch obj := result.Object.(type) {
		case *kapi.Status:
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
	storage := REST{&mockRegistry}
	build := mockBuild()
	channel, err := storage.Update(kapi.NewDefaultContext(), build)
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

func TestBuildRESTValidatesCreate(t *testing.T) {
	mockRegistry := test.BuildRegistry{}
	storage := REST{&mockRegistry}
	failureCases := map[string]api.Build{
		"empty input": {
			ObjectMeta: kapi.ObjectMeta{Name: "abc"},
			Parameters: api.BuildParameters{},
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
	mockRegistry := test.BuildRegistry{}
	storage := REST{&mockRegistry}
	failureCases := map[string]api.Build{
		"empty ID": {
			ObjectMeta: kapi.ObjectMeta{Name: ""},
			Parameters: api.BuildParameters{
				Source: api.BuildSource{
					Git: &api.GitBuildSource{
						URI: "http://my.build.com/the/build/Dockerfile",
					},
				},
				Output: api.BuildOutput{
					ImageTag: "repository/dataBuild",
				},
			},
		},
		"empty build input": {
			ObjectMeta: kapi.ObjectMeta{Name: "abc"},
			Parameters: api.BuildParameters{},
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

func mockBuild() *api.Build {
	return &api.Build{
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
		Status:  api.BuildStatusPending,
		PodName: "-the-pod-id",
	}
}

func TestCreateBuildConflictingNamespace(t *testing.T) {
	storage := REST{}

	channel, err := storage.Create(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "some-value"},
	})

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func TestUpdateBuildConflictingNamespace(t *testing.T) {
	mockRegistry := test.BuildRegistry{}
	storage := REST{&mockRegistry}

	build := mockBuild()
	channel, err := storage.Update(kapi.WithNamespace(kapi.NewContext(), "legal-name"), build)

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func checkExpectedNamespaceError(t *testing.T, err error) {
	expectedError := "Build.Namespace does not match the provided context"
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
