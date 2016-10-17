package imagestreammapping

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/context"

	etcd "github.com/coreos/etcd/client"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage/etcd/etcdtest"
	etcdtesting "k8s.io/kubernetes/pkg/storage/etcd/testing"
	"k8s.io/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/admission/testutil"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/util/restoptions"

	_ "github.com/openshift/origin/pkg/api/install"
)

var testDefaultRegistry = api.DefaultRegistryFunc(func() (string, bool) { return "defaultregistry:5000", true })

type fakeSubjectAccessReviewRegistry struct {
}

var _ subjectaccessreview.Registry = &fakeSubjectAccessReviewRegistry{}

func (f *fakeSubjectAccessReviewRegistry) CreateSubjectAccessReview(ctx kapi.Context, subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	return nil, nil
}

func setup(t *testing.T) (etcd.KeysAPI, *etcdtesting.EtcdTestServer, *REST) {

	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	etcdClient := etcd.NewKeysAPI(server.Client)

	imageStorage, err := imageetcd.NewREST(restoptions.NewSimpleGetter(etcdStorage))
	if err != nil {
		t.Fatal(err)
	}
	imageStreamStorage, imageStreamStatus, internalStorage, err := imagestreametcd.NewREST(restoptions.NewSimpleGetter(etcdStorage), testDefaultRegistry, &fakeSubjectAccessReviewRegistry{}, &testutil.FakeImageStreamLimitVerifier{})
	if err != nil {
		t.Fatal(err)
	}

	imageRegistry := image.NewRegistry(imageStorage)
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatus, internalStorage)

	storage := NewREST(imageRegistry, imageStreamRegistry, testDefaultRegistry)

	return etcdClient, server, storage
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
	_, server, storage := setup(t)
	defer server.Terminate(t)

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
	_, server, storage := setup(t)
	defer server.Terminate(t)

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
	if e, a := http.StatusNotFound, e.ErrStatus.Code; int32(e) != a {
		t.Errorf("error status code: expected %d, got %d", e, a)
	}
	if e, a := "imagestreams", e.ErrStatus.Details.Kind; e != a {
		t.Errorf("error status details kind: expected %s, got %s", e, a)
	}
	if e, a := "somerepo", e.ErrStatus.Details.Name; e != a {
		t.Errorf("error status details name: expected %s, got %s", e, a)
	}
}

func TestCreateSuccessWithName(t *testing.T) {
	client, server, storage := setup(t)
	defer server.Terminate(t)

	initialRepo := &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Namespace: "default", Name: "somerepo"},
	}

	_, err := client.Create(
		context.TODO(),
		etcdtest.AddPrefix("/imagestreams/default/somerepo"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), initialRepo),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ctx := kapi.WithUser(kapi.NewDefaultContext(), &user.DefaultInfo{})

	mapping := validNewMappingWithName()
	_, err = storage.Create(ctx, mapping)
	if err != nil {
		t.Fatalf("Unexpected error creating mapping: %#v", err)
	}

	image, err := storage.imageRegistry.GetImage(ctx, "imageID1")
	if err != nil {
		t.Errorf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.DockerImageMetadata, image.DockerImageMetadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}

	repo, err := storage.imageStreamRegistry.GetImageStream(ctx, "somerepo")
	if err != nil {
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

	client, server, storage := setup(t)
	defer server.Terminate(t)

	_, err := client.Create(
		context.TODO(),
		etcdtest.AddPrefix("/imagestreams/default/somerepo"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), existingRepo),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, err = client.Create(
		context.TODO(),
		etcdtest.AddPrefix("/images/default/"+imageID), runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), existingImage),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	mapping := api.ImageStreamMapping{
		Image: *existingImage,
		Tag:   "latest",
	}
	_, err = storage.Create(kapi.NewDefaultContext(), &mapping)
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

	client, server, storage := setup(t)
	defer server.Terminate(t)

	_, err := client.Create(
		context.TODO(),
		etcdtest.AddPrefix("/imagestreams/default/somerepo"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), existingRepo),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, err = client.Create(
		context.TODO(),
		etcdtest.AddPrefix("/images/default/existingImage"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), existingImage),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	mapping := api.ImageStreamMapping{
		Image: *existingImage,
		Tag:   "existingTag",
	}
	_, err = storage.Create(kapi.NewDefaultContext(), &mapping)
	if !errors.IsInvalid(err) {
		t.Fatalf("Unexpected non-error creating mapping: %#v", err)
	}
}

func TestTrackingTags(t *testing.T) {
	client, server, storage := setup(t)
	defer server.Terminate(t)

	stream := &api.ImageStream{
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

	_, err := client.Create(
		context.TODO(),
		etcdtest.AddPrefix("/imagestreams/default/stream"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), stream),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	image := &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "sha256:503c75e8121369581e5e5abe57b5a3f12db859052b217a8ea16eb86f4b5561a1",
		},
		DockerImageReference: "foo/bar@sha256:503c75e8121369581e5e5abe57b5a3f12db859052b217a8ea16eb86f4b5561a1",
	}

	mapping := api.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "default",
			Name:      "stream",
		},
		Image: *image,
		Tag:   "2.0",
	}

	ctx := kapi.WithUser(kapi.NewDefaultContext(), &user.DefaultInfo{})

	_, err = storage.Create(ctx, &mapping)
	if err != nil {
		t.Fatalf("Unexpected error creating mapping: %v", err)
	}

	stream, err = storage.imageStreamRegistry.GetImageStream(kapi.NewDefaultContext(), "stream")
	if err != nil {
		t.Fatalf("error extracting updated stream: %v", err)
	}

	for _, trackingTag := range []string{"tracking", "tracking2"} {
		tracking := api.LatestTaggedImage(stream, trackingTag)
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

	nonTracking := api.LatestTaggedImage(stream, "nontracking")
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

// TestCreateRetryUnrecoverable ensures that an attempt to create a mapping
// using failing registry update calls will return an error.
func TestCreateRetryUnrecoverable(t *testing.T) {
	rest := &REST{
		strategy: NewStrategy(testDefaultRegistry),
		imageRegistry: &fakeImageRegistry{
			createImage: func(ctx kapi.Context, image *api.Image) error {
				return nil
			},
		},
		imageStreamRegistry: &fakeImageStreamRegistry{
			getImageStream: func(ctx kapi.Context, id string) (*api.ImageStream, error) {
				return validImageStream(), nil
			},
			listImageStreams: func(ctx kapi.Context, options *kapi.ListOptions) (*api.ImageStreamList, error) {
				s := validImageStream()
				return &api.ImageStreamList{Items: []api.ImageStream{*s}}, nil
			},
			updateImageStreamStatus: func(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error) {
				return nil, errors.NewServiceUnavailable("unrecoverable error")
			},
		},
	}
	obj, err := rest.Create(kapi.NewDefaultContext(), validNewMappingWithName())
	if err == nil {
		t.Errorf("expected an error")
	}
	if obj != nil {
		t.Fatalf("expected a nil result")
	}
}

// TestCreateRetryConflictNoTagDiff ensures that attempts to create a mapping
// that result in resource conflicts that do NOT include tag diffs causes the
// create to be retried successfully.
func TestCreateRetryConflictNoTagDiff(t *testing.T) {
	firstUpdate := true
	rest := &REST{
		strategy: NewStrategy(testDefaultRegistry),
		imageRegistry: &fakeImageRegistry{
			createImage: func(ctx kapi.Context, image *api.Image) error {
				return nil
			},
		},
		imageStreamRegistry: &fakeImageStreamRegistry{
			getImageStream: func(ctx kapi.Context, id string) (*api.ImageStream, error) {
				stream := validImageStream()
				stream.Status = api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"latest": {Items: []api.TagEvent{{DockerImageReference: "localhost:5000/someproject/somerepo:original"}}},
					},
				}
				return stream, nil
			},
			updateImageStreamStatus: func(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error) {
				// For the first update call, return a conflict to cause a retry of an
				// image stream whose tags haven't changed.
				if firstUpdate {
					firstUpdate = false
					return nil, errors.NewConflict(api.Resource("imagestreams"), repo.Name, fmt.Errorf("resource modified"))
				}
				return repo, nil
			},
		},
	}
	obj, err := rest.Create(kapi.NewDefaultContext(), validNewMappingWithName())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if obj == nil {
		t.Fatalf("expected a result")
	}
}

// TestCreateRetryConflictTagDiff ensures that attempts to create a mapping
// that result in resource conflicts that DO contain tag diffs causes the
// conflict error to be returned.
func TestCreateRetryConflictTagDiff(t *testing.T) {
	firstGet := true
	firstUpdate := true
	rest := &REST{
		strategy: NewStrategy(testDefaultRegistry),
		imageRegistry: &fakeImageRegistry{
			createImage: func(ctx kapi.Context, image *api.Image) error {
				return nil
			},
		},
		imageStreamRegistry: &fakeImageStreamRegistry{
			getImageStream: func(ctx kapi.Context, id string) (*api.ImageStream, error) {
				// For the first get, return a stream with a latest tag pointing to "original"
				if firstGet {
					firstGet = false
					stream := validImageStream()
					stream.Status = api.ImageStreamStatus{
						Tags: map[string]api.TagEventList{
							"latest": {Items: []api.TagEvent{{DockerImageReference: "localhost:5000/someproject/somerepo:original"}}},
						},
					}
					return stream, nil
				}
				// For subsequent gets, return a stream with the latest tag changed to "newer"
				stream := validImageStream()
				stream.Status = api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"latest": {Items: []api.TagEvent{{DockerImageReference: "localhost:5000/someproject/somerepo:newer"}}},
					},
				}
				return stream, nil
			},
			updateImageStreamStatus: func(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error) {
				// For the first update, return a conflict so that the stream
				// get/compare is retried.
				if firstUpdate {
					firstUpdate = false
					return nil, errors.NewConflict(api.Resource("imagestreams"), repo.Name, fmt.Errorf("resource modified"))
				}
				return repo, nil
			},
		},
	}
	obj, err := rest.Create(kapi.NewDefaultContext(), validNewMappingWithName())
	if err == nil {
		t.Fatalf("expected an error")
	}
	if !errors.IsConflict(err) {
		t.Errorf("expected a conflict error, got %v", err)
	}
	if obj != nil {
		t.Fatalf("expected a nil result")
	}
}

type fakeImageRegistry struct {
	listImages  func(ctx kapi.Context, options *kapi.ListOptions) (*api.ImageList, error)
	getImage    func(ctx kapi.Context, id string) (*api.Image, error)
	createImage func(ctx kapi.Context, image *api.Image) error
	deleteImage func(ctx kapi.Context, id string) error
	watchImages func(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error)
	updateImage func(ctx kapi.Context, image *api.Image) (*api.Image, error)
}

func (f *fakeImageRegistry) ListImages(ctx kapi.Context, options *kapi.ListOptions) (*api.ImageList, error) {
	return f.listImages(ctx, options)
}
func (f *fakeImageRegistry) GetImage(ctx kapi.Context, id string) (*api.Image, error) {
	return f.getImage(ctx, id)
}
func (f *fakeImageRegistry) CreateImage(ctx kapi.Context, image *api.Image) error {
	return f.createImage(ctx, image)
}
func (f *fakeImageRegistry) DeleteImage(ctx kapi.Context, id string) error {
	return f.deleteImage(ctx, id)
}
func (f *fakeImageRegistry) WatchImages(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return f.watchImages(ctx, options)
}
func (f *fakeImageRegistry) UpdateImage(ctx kapi.Context, image *api.Image) (*api.Image, error) {
	return f.updateImage(ctx, image)
}

type fakeImageStreamRegistry struct {
	listImageStreams        func(ctx kapi.Context, options *kapi.ListOptions) (*api.ImageStreamList, error)
	getImageStream          func(ctx kapi.Context, id string) (*api.ImageStream, error)
	createImageStream       func(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error)
	updateImageStream       func(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error)
	updateImageStreamSpec   func(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error)
	updateImageStreamStatus func(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error)
	deleteImageStream       func(ctx kapi.Context, id string) (*unversioned.Status, error)
	watchImageStreams       func(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error)
}

func (f *fakeImageStreamRegistry) ListImageStreams(ctx kapi.Context, options *kapi.ListOptions) (*api.ImageStreamList, error) {
	return f.listImageStreams(ctx, options)
}
func (f *fakeImageStreamRegistry) GetImageStream(ctx kapi.Context, id string) (*api.ImageStream, error) {
	return f.getImageStream(ctx, id)
}
func (f *fakeImageStreamRegistry) CreateImageStream(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error) {
	return f.createImageStream(ctx, repo)
}
func (f *fakeImageStreamRegistry) UpdateImageStream(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error) {
	return f.updateImageStream(ctx, repo)
}
func (f *fakeImageStreamRegistry) UpdateImageStreamSpec(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error) {
	return f.updateImageStreamSpec(ctx, repo)
}
func (f *fakeImageStreamRegistry) UpdateImageStreamStatus(ctx kapi.Context, repo *api.ImageStream) (*api.ImageStream, error) {
	return f.updateImageStreamStatus(ctx, repo)
}
func (f *fakeImageStreamRegistry) DeleteImageStream(ctx kapi.Context, id string) (*unversioned.Status, error) {
	return f.deleteImageStream(ctx, id)
}
func (f *fakeImageStreamRegistry) WatchImageStreams(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return f.watchImageStreams(ctx, options)
}
