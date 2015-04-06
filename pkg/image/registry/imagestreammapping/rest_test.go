package imagestreammapping

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/coreos/go-etcd/etcd"

	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
)

var testDefaultRegistry = imagestream.DefaultRegistryFunc(func() (string, bool) { return "defaultregistry:5000", true })

type fakeSubjectAccessReviewRegistry struct {
}

var _ subjectaccessreview.Registry = &fakeSubjectAccessReviewRegistry{}

func (f *fakeSubjectAccessReviewRegistry) CreateSubjectAccessReview(ctx kapi.Context, subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	return nil, nil
}

func setup(t *testing.T) (*tools.FakeEtcdClient, tools.EtcdHelper, *REST) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	helper := tools.NewEtcdHelper(fakeEtcdClient, latest.Codec)
	imageStorage := imageetcd.NewREST(helper)
	imageRegistry := image.NewRegistry(imageStorage)
	imageStreamStorage, imageStreamStatus := imagestreametcd.NewREST(helper, testDefaultRegistry, &fakeSubjectAccessReviewRegistry{})
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatus)
	storage := NewREST(imageRegistry, imageStreamRegistry)
	return fakeEtcdClient, helper, storage
}

func validImageStream() *api.ImageStream {
	return &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test",
		},
	}
}

func validNewMappingWithDockerImageRepository() *api.ImageStreamMapping {
	return &api.ImageStreamMapping{
		DockerImageRepository: "localhost:5000/someproject/somerepo",
		Image: api.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "imageID1",
			},
			DockerImageReference: "localhost:5000/someproject/somerepo:imageID1",
			DockerImageMetadata: api.DockerImage{
				Config: api.DockerConfig{
					Cmd:          []string{"ls", "/"},
					Env:          []string{"a=1"},
					ExposedPorts: map[string]struct{}{"1234/tcp": {}},
					Memory:       1234,
					CPUShares:    99,
					WorkingDir:   "/workingDir",
				},
			},
		},
		Tag: "latest",
	}
}

func validNewMappingWithName() *api.ImageStreamMapping {
	return &api.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "default",
			Name:      "somerepo",
		},
		Image: api.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "imageID1",
			},
			DockerImageReference: "localhost:5000/default/somerepo:imageID1",
			DockerImageMetadata: api.DockerImage{
				Config: api.DockerConfig{
					Cmd:          []string{"ls", "/"},
					Env:          []string{"a=1"},
					ExposedPorts: map[string]struct{}{"1234/tcp": {}},
					Memory:       1234,
					CPUShares:    99,
					WorkingDir:   "/workingDir",
				},
			},
		},
		Tag: "latest",
	}
}

func TestCreateConflictingNamespace(t *testing.T) {
	_, _, storage := setup(t)

	mapping := validNewMappingWithName()
	mapping.Namespace = "some-value"

	ch, err := storage.Create(kapi.WithNamespace(kapi.NewContext(), "legal-name"), mapping)
	if ch != nil {
		t.Error("Expected a nil obj, but we got a value")
	}
	expectedError := "the namespace of the provided object does not match the namespace sent on the request"
	if err == nil {
		t.Fatalf("Expected '" + expectedError + "', but we didn't get one")
	}
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected '"+expectedError+"' error, got '%v'", err.Error())
	}
}

func TestCreateErrorListingImageStreams(t *testing.T) {
	fakeEtcdClient, _, storage := setup(t)
	fakeEtcdClient.Data["/imageRepositories/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("123"),
	}

	obj, err := storage.Create(kapi.NewDefaultContext(), validNewMappingWithDockerImageRepository())
	if obj != nil {
		t.Fatalf("Unexpected non-nil obj %#v", obj)
	}
	if err == nil {
		t.Fatal("Unexpected nil err")
	}
	if err.Error() != "123" {
		t.Errorf("Unexpeted error, got %#v", err)
	}
}

func TestCreateImageStreamNotFound(t *testing.T) {
	fakeEtcdClient, _, storage := setup(t)
	fakeEtcdClient.Data["/imageRepositories/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{
					ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "bar"},
				}),
			},
		},
	}

	obj, err := storage.Create(kapi.NewDefaultContext(), validNewMappingWithDockerImageRepository())
	if obj != nil {
		t.Errorf("Unexpected non-nil obj %#v", obj)
	}
	if err == nil {
		t.Fatal("Unexpected nil err")
	}
	if !errors.IsInvalid(err) {
		t.Fatalf("Expected 'invalid' err, got: %#v", err)
	}
}

func TestCreateSuccessWithDockerImageRepository(t *testing.T) {
	fakeEtcdClient, helper, storage := setup(t)

	initialRepo := &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "somerepo"},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "localhost:5000/someproject/somerepo",
		},
	}

	fakeEtcdClient.Data["/imageRepositories/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value:         runtime.EncodeOrDie(latest.Codec, initialRepo),
						ModifiedIndex: 1,
					},
				},
			},
		},
	}
	fakeEtcdClient.Data["/imageRepositories/default/somerepo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, initialRepo),
				ModifiedIndex: 1,
			},
		},
	}

	mapping := validNewMappingWithDockerImageRepository()
	_, err := storage.Create(kapi.NewDefaultContext(), mapping)
	if err != nil {
		t.Fatalf("Unexpected error creating mapping: %#v", err)
	}

	image := &api.Image{}
	if err := helper.ExtractObj("/images/imageID1", image, false); err != nil {
		t.Errorf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.DockerImageMetadata, image.DockerImageMetadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}

	repo := &api.ImageStream{}
	if err := helper.ExtractObj("/imageRepositories/default/somerepo", repo, false); err != nil {
		t.Errorf("Unexpected non-nil err: %#v", err)
	}
	if e, a := "imageID1", repo.Status.Tags["latest"].Items[0].Image; e != a {
		t.Errorf("unexpected repo: %#v\n%#v", repo, image)
	}
}

func TestCreateSuccessWithMismatchedNames(t *testing.T) {
	fakeEtcdClient, helper, storage := setup(t)

	initialRepo := &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "repo1"},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "localhost:5000/someproject/somerepo",
		},
	}

	fakeEtcdClient.Data["/imageRepositories/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value:         runtime.EncodeOrDie(latest.Codec, initialRepo),
						ModifiedIndex: 1,
					},
				},
			},
		},
	}
	fakeEtcdClient.Data["/imageRepositories/default/repo1"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, initialRepo),
				ModifiedIndex: 1,
			},
		},
	}

	mapping := validNewMappingWithDockerImageRepository()
	_, err := storage.Create(kapi.NewDefaultContext(), mapping)
	if err != nil {
		t.Fatalf("Unexpected error creating mapping: %#v", err)
	}

	image := &api.Image{}
	if err := helper.ExtractObj("/images/imageID1", image, false); err != nil {
		t.Errorf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.DockerImageMetadata, image.DockerImageMetadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}

	repo := &api.ImageStream{}
	if err := helper.ExtractObj("/imageRepositories/default/repo1", repo, false); err != nil {
		t.Errorf("Unexpected non-nil err: %#v", err)
	}
	if e, a := "localhost:5000/someproject/somerepo:imageID1", repo.Status.Tags["latest"].Items[0].DockerImageReference; e != a {
		t.Errorf("unexpected repo: %#v\n%#v", repo, image)
	}
	if e, a := "imageID1", repo.Status.Tags["latest"].Items[0].Image; e != a {
		t.Errorf("unexpected repo: %#v\n%#v", repo, image)
	}
}

func TestCreateSuccessWithName(t *testing.T) {
	fakeEtcdClient, helper, storage := setup(t)

	initialRepo := &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "somerepo"},
	}

	fakeEtcdClient.Data["/imageRepositories/default/somerepo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, initialRepo),
				ModifiedIndex: 1,
			},
		},
	}

	mapping := validNewMappingWithName()
	_, err := storage.Create(kapi.NewDefaultContext(), mapping)
	if err != nil {
		t.Fatalf("Unexpected error creating mapping: %#v", err)
	}

	image := &api.Image{}
	if err := helper.ExtractObj("/images/imageID1", image, false); err != nil {
		t.Errorf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.DockerImageMetadata, image.DockerImageMetadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}

	repo := &api.ImageStream{}
	if err := helper.ExtractObj("/imageRepositories/default/somerepo", repo, false); err != nil {
		t.Errorf("Unexpected non-nil err: %#v", err)
	}
	if e, a := "imageID1", repo.Status.Tags["latest"].Items[0].Image; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
}

func TestAddExistingImageWithNewTag(t *testing.T) {
	imageID := "8d812da98d6dd61620343f1a5bf6585b34ad6ed16e5c5f7c7216a525d6aeb772"
	existingRepo := &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "somerepo",
			Namespace: "default",
		},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "localhost:5000/someproject/somerepo",
			/*
				Tags: map[string]api.TagReference{
					"existingTag": {
						From: &kapi.ObjectReference{
							Kind: "ImageStreamTag",

						Tag: "existingTag", Reference: imageID},
				},
			*/
		},
		Status: api.ImageStreamStatus{
			Tags: map[string]api.TagEventList{
				"existingTag": {Items: []api.TagEvent{{DockerImageReference: "localhost:5000/someproject/somerepo:" + imageID}}},
			},
		},
	}

	existingImage := &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name:      imageID,
			Namespace: "default",
		},
		DockerImageReference: "localhost:5000/someproject/somerepo:" + imageID,
		DockerImageMetadata: api.DockerImage{
			Config: api.DockerConfig{
				Cmd:          []string{"ls", "/"},
				Env:          []string{"a=1"},
				ExposedPorts: map[string]struct{}{"1234/tcp": {}},
				Memory:       1234,
				CPUShares:    99,
				WorkingDir:   "/workingDir",
			},
		},
	}

	fakeEtcdClient, helper, storage := setup(t)
	fakeEtcdClient.Data["/imageRepositories/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value:         runtime.EncodeOrDie(latest.Codec, existingRepo),
						ModifiedIndex: 1,
					},
				},
			},
		},
	}
	fakeEtcdClient.Data["/imageRepositories/default/somerepo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, existingRepo),
				ModifiedIndex: 1,
			},
		},
	}
	fakeEtcdClient.Data["/images/default/"+imageID] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, existingImage),
				ModifiedIndex: 1,
			},
		},
	}

	mapping := api.ImageStreamMapping{
		DockerImageRepository: "localhost:5000/someproject/somerepo",
		Image: *existingImage,
		Tag:   "latest",
	}
	_, err := storage.Create(kapi.NewDefaultContext(), &mapping)
	if err != nil {
		t.Errorf("Unexpected error creating mapping: %#v", err)
	}

	image := &api.Image{}
	if err := helper.ExtractObj("/images/"+imageID, image, false); err != nil {
		t.Errorf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.DockerImageMetadata, image.DockerImageMetadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}

	repo := &api.ImageStream{}
	if err := helper.ExtractObj("/imageRepositories/default/somerepo", repo, false); err != nil {
		t.Errorf("Unexpected non-nil err: %#v", err)
	}
	/*
		if e, a := "", repo.Spec.Tags["latest"].Reference; e != a {
			t.Errorf("Expected %s, got %s", e, a)
		}
	*/
	if e, a := 2, len(repo.Status.Tags); e != a {
		t.Fatalf("repo.Status.Tags length: expected %d, got %d: %#v", e, a, repo.Status.Tags)
	}
	if e, a := 1, len(repo.Status.Tags["existingTag"].Items); e != a {
		t.Errorf("repo.Status.Tags['existingTag']: expected '%v', got '%v': %#v", e, a, repo.Status.Tags["existingTag"].Items)
	}
	if e, a := "localhost:5000/someproject/somerepo:"+imageID, repo.Status.Tags["existingTag"].Items[0].DockerImageReference; e != a {
		t.Errorf("existingTag history: expected image %s, got %s", e, a)
	}
	if e, a := 1, len(repo.Status.Tags["latest"].Items); e != a {
		t.Errorf("repo.Status.Tags['latest']: expected '%v', got '%v'", e, a)
	}
	if e, a := "localhost:5000/someproject/somerepo:"+imageID, repo.Status.Tags["latest"].Items[0].DockerImageReference; e != a {
		t.Errorf("latest history: expected image %s, got %s", e, a)
	}
	if e, a := imageID, repo.Status.Tags["latest"].Items[0].Image; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}

	event, err := api.LatestTaggedImage(repo, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.DockerImageReference != "localhost:5000/someproject/somerepo:"+imageID || event.Image != imageID {
		t.Errorf("unexpected latest tagged image: %#v", event)
	}
}

func TestAddExistingImageAndTag(t *testing.T) {
	existingRepo := &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "somerepo",
			Namespace: "default",
		},
		Spec: api.ImageStreamSpec{
			DockerImageRepository: "localhost:5000/someproject/somerepo",
			/*
				Tags: map[string]api.TagReference{
					"existingTag": {Tag: "existingTag", Reference: "existingImage"},
				},
			*/
		},
		Status: api.ImageStreamStatus{
			Tags: map[string]api.TagEventList{
				"existingTag": {Items: []api.TagEvent{{DockerImageReference: "existingImage"}}},
			},
		},
	}

	existingImage := &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "existingImage",
			Namespace: "default",
		},
		DockerImageReference: "localhost:5000/someproject/somerepo:imageID1",
		DockerImageMetadata: api.DockerImage{
			Config: api.DockerConfig{
				Cmd:          []string{"ls", "/"},
				Env:          []string{"a=1"},
				ExposedPorts: map[string]struct{}{"1234/tcp": {}},
				Memory:       1234,
				CPUShares:    99,
				WorkingDir:   "/workingDir",
			},
		},
	}

	fakeEtcdClient, helper, storage := setup(t)
	fakeEtcdClient.Data["/imageRepositories/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value:         runtime.EncodeOrDie(latest.Codec, existingRepo),
						ModifiedIndex: 1,
					},
				},
			},
		},
	}
	fakeEtcdClient.Data["/imageRepositories/default/somerepo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, existingRepo),
				ModifiedIndex: 1,
			},
		},
	}
	fakeEtcdClient.Data["/images/default/existingImage"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, existingImage),
				ModifiedIndex: 1,
			},
		},
	}

	mapping := api.ImageStreamMapping{
		DockerImageRepository: "localhost:5000/someproject/somerepo",
		Image: *existingImage,
		Tag:   "existingTag",
	}
	_, err := storage.Create(kapi.NewDefaultContext(), &mapping)
	if err != nil {
		t.Fatalf("Unexpected error creating mapping: %#v", err)
	}

	image := &api.Image{}
	if err := helper.ExtractObj("/images/existingImage", image, false); err != nil {
		t.Fatalf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.DockerImageMetadata, image.DockerImageMetadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}

	repo := &api.ImageStream{}
	if err := helper.ExtractObj("/imageRepositories/default/somerepo", repo, false); err != nil {
		t.Fatalf("Unexpected non-nil err: %#v", err)
	}
	// Tags aren't changed during mapping creation
	/*
		if e, a := "existingImage", repo.Spec.Tags["existingTag"].Reference; e != a {
			t.Errorf("Expected %s, got %s", e, a)
		}
	*/
	if e, a := 1, len(repo.Status.Tags); e != a {
		t.Errorf("repo.Status.Tags length: expected %d, got %d", e, a)
	}
	if e, a := mapping.DockerImageRepository+":imageID1", repo.Status.Tags["existingTag"].Items[0].DockerImageReference; e != a {
		t.Errorf("unexpected repo: %#v", repo)
	}
}
