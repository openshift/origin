package imagestreammapping

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	etcd "github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"

	"k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage/etcd/etcdtest"
	etcdtesting "k8s.io/apiserver/pkg/storage/etcd/testing"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/registry/registrytest"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/admission/testutil"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/util/restoptions"

	_ "github.com/openshift/origin/pkg/api/install"
)

const testDefaultRegistryURL = "defaultregistry:5000"

var testDefaultRegistry = imageapi.DefaultRegistryFunc(func() (string, bool) { return testDefaultRegistryURL, true })

type fakeSubjectAccessReviewRegistry struct {
}

var _ subjectaccessreview.Registry = &fakeSubjectAccessReviewRegistry{}

func (f *fakeSubjectAccessReviewRegistry) CreateSubjectAccessReview(ctx apirequest.Context, subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	return nil, nil
}

func setup(t *testing.T) (etcd.KV, *etcdtesting.EtcdTestServer, *REST) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	etcdClient := etcd.NewKV(server.V3Client)

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

func validImageStream() *imageapi.ImageStream {
	return &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
}

const testImageID = "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"

func validNewMappingWithName() *imageapi.ImageStreamMapping {
	return &imageapi.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "somerepo",
		},
		Image: imageapi.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name:        testImageID,
				Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
			},
			DockerImageReference: "localhost:5000/default/somerepo@" + testImageID,
			DockerImageMetadata: imageapi.DockerImage{
				Config: &imageapi.DockerConfig{
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

	ch, err := storage.Create(apirequest.WithNamespace(apirequest.NewContext(), "legal-name"), mapping, false)
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

	obj, err := storage.Create(apirequest.NewDefaultContext(), validNewMappingWithName(), false)
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

	initialRepo := &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "somerepo"},
	}

	_, err := client.Put(
		context.TODO(),
		etcdtest.AddPrefix("/imagestreams/default/somerepo"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), initialRepo),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	ctx := apirequest.WithUser(apirequest.NewDefaultContext(), &user.DefaultInfo{})

	mapping := validNewMappingWithName()
	_, err = storage.Create(ctx, mapping, false)
	if err != nil {
		t.Fatalf("Unexpected error creating mapping: %#v", err)
	}

	image, err := storage.imageRegistry.GetImage(ctx, testImageID, &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.DockerImageMetadata, image.DockerImageMetadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}

	repo, err := storage.imageStreamRegistry.GetImageStream(ctx, "somerepo", &metav1.GetOptions{})
	if err != nil {
		t.Errorf("Unexpected non-nil err: %#v", err)
	}
	if e, a := testImageID, repo.Status.Tags["latest"].Items[0].Image; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
}

func TestAddExistingImageWithNewTag(t *testing.T) {
	imageID := "sha256:8d812da98d6dd61620343f1a5bf6585b34ad6ed16e5c5f7c7216a525d6aeb772"
	existingRepo := &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "somerepo",
			Namespace: "default",
		},
		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: "localhost:5000/default/somerepo",
		},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				"existingTag": {Items: []imageapi.TagEvent{{DockerImageReference: "localhost:5000/somens/somerepo@" + imageID}}},
			},
		},
	}

	existingImage := &imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: imageID,
		},
		DockerImageReference: "localhost:5000/somens/somerepo@" + imageID,
		DockerImageMetadata: imageapi.DockerImage{
			Config: &imageapi.DockerConfig{
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

	_, err := client.Put(
		context.TODO(),
		etcdtest.AddPrefix("/imagestreams/default/somerepo"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), existingRepo),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, err = client.Put(
		context.TODO(),
		etcdtest.AddPrefix("/images/"+imageID), runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), existingImage),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	mapping := imageapi.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name: "somerepo",
		},
		Image: *existingImage,
		Tag:   "latest",
	}
	ctx := apirequest.NewDefaultContext()
	_, err = storage.Create(ctx, &mapping, false)
	if err != nil {
		t.Errorf("Unexpected error creating image stream mapping%v", err)
	}

	image, err := storage.imageRegistry.GetImage(ctx, imageID, &metav1.GetOptions{})
	if err != nil {
		t.Errorf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.DockerImageMetadata, image.DockerImageMetadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}

	repo, err := storage.imageStreamRegistry.GetImageStream(ctx, "somerepo", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected non-nil err: %#v", err)
	}
	if e, a := imageID, repo.Status.Tags["latest"].Items[0].Image; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	tagEvent := imageapi.LatestTaggedImage(repo, "latest")
	if e, a := image.DockerImageReference, tagEvent.DockerImageReference; e != a {
		t.Errorf("Unexpected tracking dockerImageReference: %q != %q", a, e)
	}

	pullSpec, ok := imageapi.ResolveLatestTaggedImage(repo, "latest")
	if !ok {
		t.Fatalf("Failed to resolv latest tagged image")
	}
	if e, a := image.DockerImageReference, pullSpec; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
}

func TestAddExistingImageOverridingDockerImageReference(t *testing.T) {
	imageID := "sha256:8d812da98d6dd61620343f1a5bf6585b34ad6ed16e5c5f7c7216a525d6aeb772"
	newRepo := &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "newrepo",
		},
		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: "localhost:5000/default/newrepo",
		},
		Status: imageapi.ImageStreamStatus{
			DockerImageRepository: "localhost:5000/default/newrepo",
		},
	}
	existingImage := &imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:        imageID,
			Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
		},
		DockerImageReference: "localhost:5000/someproject/somerepo@" + imageID,
		DockerImageMetadata: imageapi.DockerImage{
			Config: &imageapi.DockerConfig{
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

	_, err := client.Put(
		context.TODO(),
		etcdtest.AddPrefix("/imagestreams/default/newrepo"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), newRepo),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	_, err = client.Put(
		context.TODO(),
		etcdtest.AddPrefix("/images/"+imageID), runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), existingImage),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	mapping := imageapi.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name: "newrepo",
		},
		Image: *existingImage,
		Tag:   "latest",
	}
	ctx := apirequest.NewDefaultContext()
	_, err = storage.Create(ctx, &mapping, false)
	if err != nil {
		t.Fatalf("Unexpected error creating mapping: %#v", err)
	}

	image, err := storage.imageRegistry.GetImage(ctx, imageID, &metav1.GetOptions{})
	if err != nil {
		t.Errorf("Unexpected error retrieving image: %#v", err)
	}
	if e, a := mapping.Image.DockerImageReference, image.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if !reflect.DeepEqual(mapping.Image.DockerImageMetadata, image.DockerImageMetadata) {
		t.Errorf("Expected %#v, got %#v", mapping.Image, image)
	}

	repo, err := storage.imageStreamRegistry.GetImageStream(ctx, "newrepo", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected non-nil err: %#v", err)
	}
	if e, a := imageID, repo.Status.Tags["latest"].Items[0].Image; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	tagEvent := imageapi.LatestTaggedImage(repo, "latest")
	if e, a := testDefaultRegistryURL+"/default/newrepo@"+imageID, tagEvent.DockerImageReference; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
	if tagEvent.DockerImageReference == image.DockerImageReference {
		t.Errorf("Expected image stream to have dockerImageReference other than %q", image.DockerImageReference)
	}

	pullSpec, ok := imageapi.ResolveLatestTaggedImage(repo, "latest")
	if !ok {
		t.Fatalf("Failed to resolv latest tagged image")
	}
	if e, a := testDefaultRegistryURL+"/default/newrepo@"+imageID, pullSpec; e != a {
		t.Errorf("Expected %s, got %s", e, a)
	}
}

func TestAddExistingImageAndTag(t *testing.T) {
	existingRepo := &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "somerepo",
			Namespace: "default",
		},
		Spec: imageapi.ImageStreamSpec{
			DockerImageRepository: "localhost:5000/someproject/somerepo",
			/*
				Tags: map[string]imageapi.TagReference{
					"existingTag": {Tag: "existingTag", Reference: "existingImage"},
				},
			*/
		},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				"existingTag": {Items: []imageapi.TagEvent{{DockerImageReference: "existingImage"}}},
			},
		},
	}

	existingImage := &imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existingImage",
			Namespace: "default",
		},
		DockerImageReference: "localhost:5000/someproject/somerepo@" + testImageID,
		DockerImageMetadata: imageapi.DockerImage{
			Config: &imageapi.DockerConfig{
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

	_, err := client.Put(
		context.TODO(),
		etcdtest.AddPrefix("/imagestreams/default/somerepo"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), existingRepo),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, err = client.Put(
		context.TODO(),
		etcdtest.AddPrefix("/images/default/existingImage"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), existingImage),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	mapping := imageapi.ImageStreamMapping{
		Image: *existingImage,
		Tag:   "existingTag",
	}
	_, err = storage.Create(apirequest.NewDefaultContext(), &mapping, false)
	if !errors.IsInvalid(err) {
		t.Fatalf("Unexpected non-error creating mapping: %#v", err)
	}
}

func TestTrackingTags(t *testing.T) {
	client, server, storage := setup(t)
	defer server.Terminate(t)

	stream := &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "stream",
		},
		Spec: imageapi.ImageStreamSpec{
			Tags: map[string]imageapi.TagReference{
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
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				"tracking": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "foo/bar@sha256:1234",
							Image:                "1234",
						},
					},
				},
				"nontracking": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "bar/baz@sha256:9999",
							Image:                "9999",
						},
					},
				},
				"2.0": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "foo/bar@sha256:1234",
							Image:                "1234",
						},
					},
				},
			},
		},
	}

	_, err := client.Put(
		context.TODO(),
		etcdtest.AddPrefix("/imagestreams/default/stream"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), stream),
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	image := &imageapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sha256:503c75e8121369581e5e5abe57b5a3f12db859052b217a8ea16eb86f4b5561a1",
		},
		DockerImageReference: "foo/bar@sha256:503c75e8121369581e5e5abe57b5a3f12db859052b217a8ea16eb86f4b5561a1",
	}

	mapping := imageapi.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "stream",
		},
		Image: *image,
		Tag:   "2.0",
	}

	ctx := apirequest.WithUser(apirequest.NewDefaultContext(), &user.DefaultInfo{})

	_, err = storage.Create(ctx, &mapping, false)
	if err != nil {
		t.Fatalf("Unexpected error creating mapping: %v", err)
	}

	stream, err = storage.imageStreamRegistry.GetImageStream(apirequest.NewDefaultContext(), "stream", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error extracting updated stream: %v", err)
	}

	for _, trackingTag := range []string{"tracking", "tracking2"} {
		tracking := imageapi.LatestTaggedImage(stream, trackingTag)
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

	nonTracking := imageapi.LatestTaggedImage(stream, "nontracking")
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
			createImage: func(ctx apirequest.Context, image *imageapi.Image) error {
				return nil
			},
		},
		imageStreamRegistry: &fakeImageStreamRegistry{
			getImageStream: func(ctx apirequest.Context, id string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
				return validImageStream(), nil
			},
			listImageStreams: func(ctx apirequest.Context, options *metainternal.ListOptions) (*imageapi.ImageStreamList, error) {
				s := validImageStream()
				return &imageapi.ImageStreamList{Items: []imageapi.ImageStream{*s}}, nil
			},
			updateImageStreamStatus: func(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error) {
				return nil, errors.NewServiceUnavailable("unrecoverable error")
			},
		},
	}
	obj, err := rest.Create(apirequest.NewDefaultContext(), validNewMappingWithName(), false)
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
			createImage: func(ctx apirequest.Context, image *imageapi.Image) error {
				return nil
			},
		},
		imageStreamRegistry: &fakeImageStreamRegistry{
			getImageStream: func(ctx apirequest.Context, id string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
				stream := validImageStream()
				stream.Status = imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {Items: []imageapi.TagEvent{{DockerImageReference: "localhost:5000/someproject/somerepo:original"}}},
					},
				}
				return stream, nil
			},
			updateImageStreamStatus: func(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error) {
				// For the first update call, return a conflict to cause a retry of an
				// image stream whose tags haven't changed.
				if firstUpdate {
					firstUpdate = false
					return nil, errors.NewConflict(imageapi.Resource("imagestreams"), repo.Name, fmt.Errorf("resource modified"))
				}
				return repo, nil
			},
		},
	}
	obj, err := rest.Create(apirequest.NewDefaultContext(), validNewMappingWithName(), false)
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
			createImage: func(ctx apirequest.Context, image *imageapi.Image) error {
				return nil
			},
		},
		imageStreamRegistry: &fakeImageStreamRegistry{
			getImageStream: func(ctx apirequest.Context, id string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
				// For the first get, return a stream with a latest tag pointing to "original"
				if firstGet {
					firstGet = false
					stream := validImageStream()
					stream.Status = imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {Items: []imageapi.TagEvent{{DockerImageReference: "localhost:5000/someproject/somerepo:original"}}},
						},
					}
					return stream, nil
				}
				// For subsequent gets, return a stream with the latest tag changed to "newer"
				stream := validImageStream()
				stream.Status = imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {Items: []imageapi.TagEvent{{DockerImageReference: "localhost:5000/someproject/somerepo:newer"}}},
					},
				}
				return stream, nil
			},
			updateImageStreamStatus: func(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error) {
				// For the first update, return a conflict so that the stream
				// get/compare is retried.
				if firstUpdate {
					firstUpdate = false
					return nil, errors.NewConflict(imageapi.Resource("imagestreams"), repo.Name, fmt.Errorf("resource modified"))
				}
				return repo, nil
			},
		},
	}
	obj, err := rest.Create(apirequest.NewDefaultContext(), validNewMappingWithName(), false)
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
	listImages  func(ctx apirequest.Context, options *metainternal.ListOptions) (*imageapi.ImageList, error)
	getImage    func(ctx apirequest.Context, id string, options *metav1.GetOptions) (*imageapi.Image, error)
	createImage func(ctx apirequest.Context, image *imageapi.Image) error
	deleteImage func(ctx apirequest.Context, id string) error
	watchImages func(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
	updateImage func(ctx apirequest.Context, image *imageapi.Image) (*imageapi.Image, error)
}

func (f *fakeImageRegistry) ListImages(ctx apirequest.Context, options *metainternal.ListOptions) (*imageapi.ImageList, error) {
	return f.listImages(ctx, options)
}
func (f *fakeImageRegistry) GetImage(ctx apirequest.Context, id string, options *metav1.GetOptions) (*imageapi.Image, error) {
	return f.getImage(ctx, id, options)
}
func (f *fakeImageRegistry) CreateImage(ctx apirequest.Context, image *imageapi.Image) error {
	return f.createImage(ctx, image)
}
func (f *fakeImageRegistry) DeleteImage(ctx apirequest.Context, id string) error {
	return f.deleteImage(ctx, id)
}
func (f *fakeImageRegistry) WatchImages(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return f.watchImages(ctx, options)
}
func (f *fakeImageRegistry) UpdateImage(ctx apirequest.Context, image *imageapi.Image) (*imageapi.Image, error) {
	return f.updateImage(ctx, image)
}

type fakeImageStreamRegistry struct {
	listImageStreams        func(ctx apirequest.Context, options *metainternal.ListOptions) (*imageapi.ImageStreamList, error)
	getImageStream          func(ctx apirequest.Context, id string, options *metav1.GetOptions) (*imageapi.ImageStream, error)
	createImageStream       func(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error)
	updateImageStream       func(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error)
	updateImageStreamSpec   func(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error)
	updateImageStreamStatus func(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error)
	deleteImageStream       func(ctx apirequest.Context, id string) (*metav1.Status, error)
	watchImageStreams       func(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
}

func (f *fakeImageStreamRegistry) ListImageStreams(ctx apirequest.Context, options *metainternal.ListOptions) (*imageapi.ImageStreamList, error) {
	return f.listImageStreams(ctx, options)
}
func (f *fakeImageStreamRegistry) GetImageStream(ctx apirequest.Context, id string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
	return f.getImageStream(ctx, id, options)
}
func (f *fakeImageStreamRegistry) CreateImageStream(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	return f.createImageStream(ctx, repo)
}
func (f *fakeImageStreamRegistry) UpdateImageStream(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	return f.updateImageStream(ctx, repo)
}
func (f *fakeImageStreamRegistry) UpdateImageStreamSpec(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	return f.updateImageStreamSpec(ctx, repo)
}
func (f *fakeImageStreamRegistry) UpdateImageStreamStatus(ctx apirequest.Context, repo *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	return f.updateImageStreamStatus(ctx, repo)
}
func (f *fakeImageStreamRegistry) DeleteImageStream(ctx apirequest.Context, id string) (*metav1.Status, error) {
	return f.deleteImageStream(ctx, id)
}
func (f *fakeImageStreamRegistry) WatchImageStreams(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return f.watchImageStreams(ctx, options)
}
