package imagerepositorytag

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/coreos/go-etcd/etcd"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
	imagerepositoryetcd "github.com/openshift/origin/pkg/image/registry/imagerepository/etcd"
)

var testDefaultRegistry = imagerepository.DefaultRegistryFunc(func() (string, bool) { return "defaultregistry:5000", true })

func setup(t *testing.T) (*tools.FakeEtcdClient, tools.EtcdHelper, *REST) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	helper := tools.NewEtcdHelper(fakeEtcdClient, latest.Codec)
	imageStorage := imageetcd.NewREST(helper)
	imageRegistry := image.NewRegistry(imageStorage)
	imageRepositoryStorage, imageRepositoryStatus := imagerepositoryetcd.NewREST(helper, testDefaultRegistry)
	imageRepositoryRegistry := imagerepository.NewRegistry(imageRepositoryStorage, imageRepositoryStatus)
	storage := NewREST(imageRegistry, imageRepositoryRegistry)
	return fakeEtcdClient, helper, storage
}

type statusError interface {
	Status() kapi.Status
}

func TestNameAndTag(t *testing.T) {
	tests := map[string]struct {
		id           string
		expectedName string
		expectedTag  string
		expectError  bool
	}{
		"empty id": {
			id:          "",
			expectError: true,
		},
		"missing semicolon": {
			id:          "hello",
			expectError: true,
		},
		"too many semicolons": {
			id:          "a:b:c",
			expectError: true,
		},
		"empty name": {
			id:          ":tag",
			expectError: true,
		},
		"empty tag": {
			id:          "name",
			expectError: true,
		},
		"happy path": {
			id:           "name:tag",
			expectError:  false,
			expectedName: "name",
			expectedTag:  "tag",
		},
	}

	for description, testCase := range tests {
		name, tag, err := nameAndTag(testCase.id)
		gotError := err != nil
		if e, a := testCase.expectError, gotError; e != a {
			t.Fatalf("%s: expected err: %t, got: %t: %s", description, e, a, err)
		}
		if err != nil {
			continue
		}
		if e, a := testCase.expectedName, name; e != a {
			t.Errorf("%s: name: expected %q, got %q", description, e, a)
		}
		if e, a := testCase.expectedTag, tag; e != a {
			t.Errorf("%s: tag: expected %q, got %q", description, e, a)
		}
	}
}

func TestGetImageRepositoryTag(t *testing.T) {
	tests := map[string]struct {
		image           *api.Image
		repo            *api.ImageRepository
		expectError     bool
		errorTargetKind string
		errorTargetID   string
	}{
		"happy path": {
			image: &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
			repo: &api.ImageRepository{Status: api.ImageRepositoryStatus{
				Tags: map[string]api.TagEventList{
					"latest": {Items: []api.TagEvent{{DockerImageReference: "test", Image: "10"}}},
				},
			}},
		},
		"synthetic image from partial tag": {
			image: &api.Image{ObjectMeta: kapi.ObjectMeta{Name: ""}, DockerImageReference: "test"},
			repo: &api.ImageRepository{Status: api.ImageRepositoryStatus{
				Tags: map[string]api.TagEventList{
					"latest": {Items: []api.TagEvent{{DockerImageReference: "test", Image: ""}}},
				},
			}},
		},
		"tag event reference required": {
			image: &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
			repo: &api.ImageRepository{Status: api.ImageRepositoryStatus{
				Tags: map[string]api.TagEventList{
					"latest": {Items: []api.TagEvent{{Image: "10"}}},
				},
			}},
			expectError:     true,
			errorTargetKind: "imageRepositoryTag",
			errorTargetID:   "latest",
		},
		"missing image": {
			repo: &api.ImageRepository{Status: api.ImageRepositoryStatus{
				Tags: map[string]api.TagEventList{
					"latest": {Items: []api.TagEvent{{DockerImageReference: "test", Image: "10"}}},
				},
			}},
			expectError:     true,
			errorTargetKind: "image",
			errorTargetID:   "10",
		},
		"missing repo": {
			expectError:     true,
			errorTargetKind: "imageRepository",
			errorTargetID:   "test",
		},
		"missing tag": {
			image: &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
			repo: &api.ImageRepository{Status: api.ImageRepositoryStatus{
				Tags: map[string]api.TagEventList{
					"other": {Items: []api.TagEvent{{DockerImageReference: "test", Image: "10"}}},
				},
			}},
			expectError:     true,
			errorTargetKind: "imageRepositoryTag",
			errorTargetID:   "latest",
		},
	}

	for name, testCase := range tests {
		fakeEtcdClient, _, storage := setup(t)

		if testCase.image != nil {
			fakeEtcdClient.Data["/images/"+testCase.image.Name] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: &etcd.Node{
						Value:         runtime.EncodeOrDie(latest.Codec, testCase.image),
						ModifiedIndex: 1,
					},
				},
			}
		} else {
			fakeEtcdClient.Data["/images/10"] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: nil,
				},
				E: tools.EtcdErrorNotFound,
			}
		}

		if testCase.repo != nil {
			fakeEtcdClient.Data["/imageRepositories/default/test"] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: &etcd.Node{
						Value:         runtime.EncodeOrDie(latest.Codec, testCase.repo),
						ModifiedIndex: 1,
					},
				},
			}
		} else {
			fakeEtcdClient.Data["/imageRepositories/default/test"] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: nil,
				},
				E: tools.EtcdErrorNotFound,
			}
		}

		obj, err := storage.Get(kapi.NewDefaultContext(), "test:latest")
		gotErr := err != nil
		if e, a := testCase.expectError, gotErr; e != a {
			t.Fatalf("%s: Expected err=%v: got %v: %v", name, e, a, err)
		}
		if testCase.expectError {
			if !errors.IsNotFound(err) {
				t.Fatalf("%s: unexpected error type: %v", name, err)
			}
			status := err.(statusError).Status()
			if status.Details.Kind != testCase.errorTargetKind || status.Details.ID != testCase.errorTargetID {
				t.Errorf("%s: unexpected status: %#v", name, status)
			}
		} else {
			actual := obj.(*api.Image)
			if e, a := testCase.image.Name, actual.Name; e != a {
				t.Errorf("%s: image name: expected %v, got %v", name, e, a)
			}
		}
	}
}

func TestDeleteImageRepositoryTag(t *testing.T) {
	tests := map[string]struct {
		repo        *api.ImageRepository
		expectError bool
	}{
		"repo not found": {
			expectError: true,
		},
		"nil tag map": {
			repo: &api.ImageRepository{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
			},
			expectError: true,
		},
		"missing tag": {
			repo: &api.ImageRepository{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Tags: map[string]string{"other": "10"},
			},
			expectError: true,
		},
		"happy path": {
			repo: &api.ImageRepository{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Tags: map[string]string{"latest": "10", "another": "20"},
			},
		},
	}

	for name, testCase := range tests {
		fakeEtcdClient, helper, storage := setup(t)
		if testCase.repo != nil {
			fakeEtcdClient.Data["/imageRepositories/default/test"] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: &etcd.Node{
						Value:         runtime.EncodeOrDie(latest.Codec, testCase.repo),
						ModifiedIndex: 1,
					},
				},
			}
		} else {
			fakeEtcdClient.Data["/imageRepositories/default/test"] = tools.EtcdResponseWithError{
				R: &etcd.Response{
					Node: nil,
				},
				E: tools.EtcdErrorNotFound,
			}
		}

		obj, err := storage.Delete(kapi.NewDefaultContext(), "test:latest")
		gotError := err != nil
		if e, a := testCase.expectError, gotError; e != a {
			t.Fatalf("%s: expectError=%t, gotError=%t: %s", name, e, a, err)
		}
		if testCase.expectError {
			continue
		}

		if obj == nil {
			t.Fatalf("%s: unexpected nil response", name)
		}
		expectedStatus := &kapi.Status{Status: kapi.StatusSuccess}
		if e, a := expectedStatus, obj; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: expected %#v, got %#v", name, e, a)
		}

		updatedRepo := &api.ImageRepository{}
		if err := helper.ExtractObj("/imageRepositories/default/test", updatedRepo, false); err != nil {
			t.Fatalf("%s: error retrieving updated repo: %s", name, err)
		}
		if e, a := map[string]string{"another": "20"}, updatedRepo.Tags; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: tags: expected %v, got %v", name, e, a)
		}
	}
}
