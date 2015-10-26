package imagestreammapping

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/coreos/go-etcd/etcd"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/runtime"
	kstorage "k8s.io/kubernetes/pkg/storage"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	"k8s.io/kubernetes/pkg/tools"
	"k8s.io/kubernetes/pkg/tools/etcdtest"

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

func setup(t *testing.T) (*tools.FakeEtcdClient, kstorage.Interface, *REST) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	helper := etcdstorage.NewEtcdStorage(fakeEtcdClient, latest.Codec, etcdtest.PathPrefix())
	imageStorage := imageetcd.NewREST(helper)
	imageRegistry := image.NewRegistry(imageStorage)
	imageStreamStorage, imageStreamStatus, internalStorage := imagestreametcd.NewREST(helper, testDefaultRegistry, &fakeSubjectAccessReviewRegistry{})
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatus, internalStorage)
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
				Config: &api.DockerConfig{
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

func TestCreateImageStreamNotFoundWithName(t *testing.T) {
	fakeEtcdClient, _, storage := setup(t)
	fakeEtcdClient.ExpectNotFoundGet("/imagestreams/default/somerepo")

	obj, err := storage.Create(kapi.NewDefaultContext(), validNewMappingWithName())
	if obj != nil {
		t.Errorf("Unexpected non-nil obj %#v", obj)
	}
	if err == nil {
		t.Fatal("Unexpected nil err")
	}
	e, ok := err.(*errors.StatusError)
	if !ok {
		t.Fatalf("expected StatusError, got %#v", err)
	}
	if e, a := http.StatusNotFound, e.ErrStatus.Code; e != a {
		t.Errorf("error status code: expected %d, got %d", e, a)
	}
	if e, a := "imageStream", e.ErrStatus.Details.Kind; e != a {
		t.Errorf("error status details kind: expected %s, got %s", e, a)
	}
	if e, a := "somerepo", e.ErrStatus.Details.Name; e != a {
		t.Errorf("error status details name: expected %s, got %s", e, a)
	}
}

func TestCreateSuccessWithName(t *testing.T) {
	fakeEtcdClient, helper, storage := setup(t)

	initialRepo := &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "somerepo"},
	}

	fakeEtcdClient.Data[etcdtest.AddPrefix("/imagestreams/default/somerepo")] = tools.EtcdResponseWithError{
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
	if err := helper.Get(kapi.NewDefaultContext(), "/images/imageID1", image, false); err != nil {
		t.Errorf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.DockerImageMetadata, image.DockerImageMetadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}

	repo := &api.ImageStream{}
	if err := helper.Get(kapi.NewDefaultContext(), "/imagestreams/default/somerepo", repo, false); err != nil {
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
			Config: &api.DockerConfig{
				Cmd:          []string{"ls", "/"},
				Env:          []string{"a=1"},
				ExposedPorts: map[string]struct{}{"1234/tcp": {}},
				Memory:       1234,
				CPUShares:    99,
				WorkingDir:   "/workingDir",
			},
		},
	}

	fakeEtcdClient, _, storage := setup(t)
	fakeEtcdClient.Data[etcdtest.AddPrefix("/imagestreams/default")] = tools.EtcdResponseWithError{
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
	fakeEtcdClient.Data[etcdtest.AddPrefix("/imagestreams/default/somerepo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, existingRepo),
				ModifiedIndex: 1,
			},
		},
	}
	fakeEtcdClient.Data[etcdtest.AddPrefix("/images/default/"+imageID)] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, existingImage),
				ModifiedIndex: 1,
			},
		},
	}

	mapping := api.ImageStreamMapping{
		Image: *existingImage,
		Tag:   "latest",
	}
	_, err := storage.Create(kapi.NewDefaultContext(), &mapping)
	if !errors.IsInvalid(err) {
		t.Fatalf("Unexpected non-error creating mapping: %#v", err)
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
			Config: &api.DockerConfig{
				Cmd:          []string{"ls", "/"},
				Env:          []string{"a=1"},
				ExposedPorts: map[string]struct{}{"1234/tcp": {}},
				Memory:       1234,
				CPUShares:    99,
				WorkingDir:   "/workingDir",
			},
		},
	}

	fakeEtcdClient, _, storage := setup(t)
	fakeEtcdClient.Data[etcdtest.AddPrefix("/imagestreams/default")] = tools.EtcdResponseWithError{
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
	fakeEtcdClient.Data[etcdtest.AddPrefix("/imagestreams/default/somerepo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, existingRepo),
				ModifiedIndex: 1,
			},
		},
	}
	fakeEtcdClient.Data[etcdtest.AddPrefix("/images/default/existingImage")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, existingImage),
				ModifiedIndex: 1,
			},
		},
	}

	mapping := api.ImageStreamMapping{
		Image: *existingImage,
		Tag:   "existingTag",
	}
	_, err := storage.Create(kapi.NewDefaultContext(), &mapping)
	if !errors.IsInvalid(err) {
		t.Fatalf("Unexpected non-error creating mapping: %#v", err)
	}
}

func TestTrackingTags(t *testing.T) {
	etcdClient, etcdHelper, storage := setup(t)
	_ = etcdClient

	stream := api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "default",
			Name:      "stream",
		},
		Spec: api.ImageStreamSpec{
			Tags: map[string]api.TagReference{
				"tracking": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "2.0",
					},
				},
				"tracking2": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "2.0",
					},
				},
			},
		},
		Status: api.ImageStreamStatus{
			Tags: map[string]api.TagEventList{
				"tracking": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "foo/bar@sha256:1234",
							Image:                "1234",
						},
					},
				},
				"nontracking": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "bar/baz@sha256:9999",
							Image:                "9999",
						},
					},
				},
				"2.0": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "foo/bar@sha256:1234",
							Image:                "1234",
						},
					},
				},
			},
		},
	}

	if err := etcdHelper.Create(kapi.NewDefaultContext(), "/imagestreams/default/stream", &stream, nil, 0); err != nil {
		t.Fatalf("Unable to create stream: %v", err)
	}

	image := &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "5678",
		},
		DockerImageReference: "foo/bar@sha256:5678",
	}

	mapping := api.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "default",
			Name:      "stream",
		},
		Image: *image,
		Tag:   "2.0",
	}

	_, err := storage.Create(kapi.NewDefaultContext(), &mapping)
	if err != nil {
		t.Fatalf("Unexpected error creating mapping: %v", err)
	}

	if err := etcdHelper.Get(kapi.NewDefaultContext(), "/imagestreams/default/stream", &stream, false); err != nil {
		t.Fatalf("error extracting updated stream: %v", err)
	}

	for _, trackingTag := range []string{"tracking", "tracking2"} {
		tracking := api.LatestTaggedImage(&stream, trackingTag)
		if tracking == nil {
			t.Fatalf("unexpected nil %s TagEvent", trackingTag)
		}

		if e, a := image.DockerImageReference, tracking.DockerImageReference; e != a {
			t.Errorf("dockerImageReference: expected %s, got %s", e, a)
		}
		if e, a := image.Name, tracking.Image; e != a {
			t.Errorf("image: expected %s, got %s", e, a)
		}
	}

	nonTracking := api.LatestTaggedImage(&stream, "nontracking")
	if nonTracking == nil {
		t.Fatal("unexpected nil nontracking TagEvent")
	}

	if e, a := "bar/baz@sha256:9999", nonTracking.DockerImageReference; e != a {
		t.Errorf("dockerImageReference: expected %s, got %s", e, a)
	}
	if e, a := "9999", nonTracking.Image; e != a {
		t.Errorf("image: expected %s, got %s", e, a)
	}
}
