package imagestreamtag

import (
	"reflect"
	"testing"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
	"k8s.io/apiserver/pkg/registry/rest"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/storage/etcd/etcdtest"
	etcdtesting "k8s.io/apiserver/pkg/storage/etcd/testing"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	authorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/registry/registrytest"

	admfake "github.com/openshift/origin/pkg/image/admission/fake"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation/fake"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/util/restoptions"

	_ "github.com/openshift/origin/pkg/api/install"
)

var testDefaultRegistry = func() (string, bool) { return "defaultregistry:5000", true }

type fakeSubjectAccessReviewRegistry struct {
}

func (f *fakeSubjectAccessReviewRegistry) Create(subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReview, error) {
	return nil, nil
}

type fakeUser struct {
}

var _ user.Info = &fakeUser{}

func (u *fakeUser) GetName() string {
	return "user"
}

func (u *fakeUser) GetUID() string {
	return "uid"
}

func (u *fakeUser) GetGroups() []string {
	return []string{"group1"}
}

func (u *fakeUser) GetExtra() map[string][]string {
	return map[string][]string{}
}

func setup(t *testing.T) (etcd.KV, *etcdtesting.EtcdTestServer, *REST) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	etcdClient := etcd.NewKV(server.V3Client)
	rw := &fake.RegistryWhitelister{}

	imageStorage, err := imageetcd.NewREST(restoptions.NewSimpleGetter(etcdStorage))
	if err != nil {
		t.Fatal(err)
	}
	registry := imageapi.DefaultRegistryHostnameRetriever(testDefaultRegistry, "", "")
	imageStreamStorage, imageStreamStatus, internalStorage, err := imagestreametcd.NewREST(
		restoptions.NewSimpleGetter(etcdStorage),
		registry,
		&fakeSubjectAccessReviewRegistry{},
		&admfake.ImageStreamLimitVerifier{},
		rw)
	if err != nil {
		t.Fatal(err)
	}

	imageRegistry := image.NewRegistry(imageStorage)
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatus, internalStorage)

	storage := NewREST(imageRegistry, imageStreamRegistry, rw)

	return etcdClient, server, storage
}

type statusError interface {
	Status() metav1.Status
}

func TestGetImageStreamTag(t *testing.T) {
	tests := map[string]struct {
		image           *imageapi.Image
		repo            *imageapi.ImageStream
		expectError     bool
		errorTargetKind string
		errorTargetID   string
	}{
		"happy path": {
			image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
			repo: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {
							Annotations: map[string]string{
								"color": "blue",
								"size":  "large",
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									Created:              metav1.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
									DockerImageReference: "test",
									Image:                "10",
								},
							},
						},
					},
				},
			},
		},
		"image = ''": {
			repo: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {Items: []imageapi.TagEvent{{DockerImageReference: "test", Image: ""}}},
					},
				}},
			expectError:     true,
			errorTargetKind: "imagestreamtags",
			errorTargetID:   "test:latest",
		},
		"missing image": {
			repo: &imageapi.ImageStream{Status: imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {Items: []imageapi.TagEvent{{DockerImageReference: "test", Image: "10"}}},
				},
			}},
			expectError:     true,
			errorTargetKind: "images",
			errorTargetID:   "10",
		},
		"missing repo": {
			expectError:     true,
			errorTargetKind: "imagestreams",
			errorTargetID:   "test",
		},
		"missing tag": {
			image: &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
			repo: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"other": {Items: []imageapi.TagEvent{{DockerImageReference: "test", Image: "10"}}},
					},
				}},
			expectError:     true,
			errorTargetKind: "imagestreamtags",
			errorTargetID:   "test:latest",
		},
	}

	for name, testCase := range tests {
		func() {
			client, server, storage := setup(t)
			defer server.Terminate(t)

			if testCase.image != nil {
				client.Put(
					context.TODO(),
					etcdtest.AddPrefix("/images/"+testCase.image.Name),
					runtime.EncodeOrDie(legacyscheme.Codecs.LegacyCodec(v1.SchemeGroupVersion), testCase.image),
				)
			}
			if testCase.repo != nil {
				client.Put(
					context.TODO(),
					etcdtest.AddPrefix("/imagestreams/default/test"),
					runtime.EncodeOrDie(legacyscheme.Codecs.LegacyCodec(v1.SchemeGroupVersion), testCase.repo),
				)
			}

			obj, err := storage.Get(apirequest.NewDefaultContext(), "test:latest", &metav1.GetOptions{})
			gotErr := err != nil
			if e, a := testCase.expectError, gotErr; e != a {
				t.Errorf("%s: Expected err=%v: got %v: %v", name, e, a, err)
				return
			}
			if testCase.expectError {
				if !errors.IsNotFound(err) {
					t.Errorf("%s: unexpected error type: %v", name, err)
					return
				}
				status := err.(statusError).Status()
				if status.Details.Kind != testCase.errorTargetKind || status.Details.Name != testCase.errorTargetID {
					t.Errorf("%s: unexpected status: %#v", name, status.Details)
					return
				}
			} else {
				actual := obj.(*imageapi.ImageStreamTag)
				if e, a := "default", actual.Namespace; e != a {
					t.Errorf("%s: namespace: expected %v, got %v", name, e, a)
				}
				if e, a := "test:latest", actual.Name; e != a {
					t.Errorf("%s: name: expected %v, got %v", name, e, a)
				}
				if e, a := map[string]string{"size": "large", "color": "blue"}, actual.Image.Annotations; !reflect.DeepEqual(e, a) {
					t.Errorf("%s: annotations: expected %v, got %v", name, e, a)
				}
				if e, a := metav1.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC), actual.CreationTimestamp; !a.Equal(&e) {
					t.Errorf("%s: timestamp: expected %v, got %v", name, e, a)
				}
			}
		}()
	}
}

func TestGetImageStreamTagDIR(t *testing.T) {
	expDockerImageReference := "foo/bar/baz:latest"
	image := &imageapi.Image{ObjectMeta: metav1.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz:different"}
	repo := &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				"latest": {
					Items: []imageapi.TagEvent{
						{
							Created:              metav1.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
							DockerImageReference: expDockerImageReference,
							Image:                "10",
						},
					},
				},
			},
		},
	}

	client, server, storage := setup(t)
	defer server.Terminate(t)
	client.Put(
		context.TODO(),
		etcdtest.AddPrefix("/images/"+image.Name),
		runtime.EncodeOrDie(legacyscheme.Codecs.LegacyCodec(v1.SchemeGroupVersion), image),
	)
	client.Put(
		context.TODO(),
		etcdtest.AddPrefix("/imagestreams/default/test"),
		runtime.EncodeOrDie(legacyscheme.Codecs.LegacyCodec(v1.SchemeGroupVersion), repo),
	)
	obj, err := storage.Get(apirequest.NewDefaultContext(), "test:latest", &metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	actual := obj.(*imageapi.ImageStreamTag)
	if actual.Image.DockerImageReference != expDockerImageReference {
		t.Errorf("Different DockerImageReference: expected %s, got %s", expDockerImageReference, actual.Image.DockerImageReference)
	}
}

func TestDeleteImageStreamTag(t *testing.T) {
	tests := map[string]struct {
		repo        *imageapi.ImageStream
		expectError bool
	}{
		"repo not found": {
			expectError: true,
		},
		"nil tag map": {
			repo: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
			},
			expectError: true,
		},
		"missing tag": {
			repo: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"other": {
							From: &kapi.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "test:foo",
							},
						},
					},
				},
			},
			expectError: true,
		},
		"happy path": {
			repo: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:  "default",
					Name:       "test",
					Generation: 2,
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"another": {
							From: &kapi.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "test:foo",
							},
						},
						"latest": {
							From: &kapi.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "test:bar",
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					DockerImageRepository: "registry.default.local/default/test",
					Tags: map[string]imageapi.TagEventList{
						"another": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Generation:           2,
								},
							},
						},
						"foo": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Generation:           2,
								},
							},
						},
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Generation:           2,
								},
							},
						},
						"bar": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Generation:           2,
								},
							},
						},
					},
				},
			},
		},
	}

	for name, testCase := range tests {
		func() {
			client, server, storage := setup(t)
			defer server.Terminate(t)

			if testCase.repo != nil {
				client.Put(
					context.TODO(),
					etcdtest.AddPrefix("/imagestreams/default/test"),
					runtime.EncodeOrDie(legacyscheme.Codecs.LegacyCodec(v1.SchemeGroupVersion), testCase.repo),
				)
			}

			ctx := apirequest.WithUser(apirequest.NewDefaultContext(), &fakeUser{})
			obj, err := storage.Delete(ctx, "test:latest")
			gotError := err != nil
			if e, a := testCase.expectError, gotError; e != a {
				t.Fatalf("%s: expectError=%t, gotError=%t: %s", name, e, a, err)
			}
			if testCase.expectError {
				return
			}

			if obj == nil {
				t.Fatalf("%s: unexpected nil response", name)
			}
			expectedStatus := &metav1.Status{Status: metav1.StatusSuccess}
			if e, a := expectedStatus, obj; !reflect.DeepEqual(e, a) {
				t.Errorf("%s:\nexpect=%#v\nactual=%#v", name, e, a)
			}

			updatedRepo, err := storage.imageStreamRegistry.GetImageStream(apirequest.NewDefaultContext(), "test", &metav1.GetOptions{})
			if err != nil {
				t.Fatalf("%s: error retrieving updated repo: %s", name, err)
			}
			three := int64(3)
			expectedStreamSpec := map[string]imageapi.TagReference{
				"another": {
					Name: "another",
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "test:foo",
					},
					Generation: &three,
					ReferencePolicy: imageapi.TagReferencePolicy{
						Type: imageapi.SourceTagReferencePolicy,
					},
				},
			}
			expectedStreamStatus := map[string]imageapi.TagEventList{
				"another": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
							Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
							Generation:           2,
						},
					},
				},
				"foo": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
							Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
							Generation:           2,
						},
					},
				},
				"bar": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
							Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
							Generation:           2,
						},
					},
				},
			}

			if updatedRepo.Generation != 3 {
				t.Errorf("%s: unexpected generation: %d", name, updatedRepo.Generation)
			}
			if e, a := expectedStreamStatus, updatedRepo.Status.Tags; !reflect.DeepEqual(e, a) {
				t.Errorf("%s: stream spec:\nexpect=%#v\nactual=%#v", name, e, a)
			}
			if e, a := expectedStreamSpec, updatedRepo.Spec.Tags; !reflect.DeepEqual(e, a) {
				t.Errorf("%s: stream spec:\nexpect=%#v\nactual=%#v", name, e, a)
			}
		}()
	}
}

func TestCreateImageStreamTag(t *testing.T) {
	tests := map[string]struct {
		istag           runtime.Object
		expectError     bool
		errorTargetKind string
		errorTargetID   string
	}{
		"valid istag": {
			istag: &imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test:tag",
				},
				Image: imageapi.Image{ObjectMeta: metav1.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
				Tag: &imageapi.TagReference{
					Name:            "latest",
					From:            &kapi.ObjectReference{Kind: "DockerImage", Name: "foo/bar/baz"},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.SourceTagReferencePolicy},
				},
			},
		},
		"invalid tag": {
			istag: &imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test:tag",
				},
				Image: imageapi.Image{ObjectMeta: metav1.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
				Tag:   &imageapi.TagReference{},
			},
			expectError:     true,
			errorTargetKind: "ImageStreamTag",
			errorTargetID:   "test:tag",
		},
		"nil tag": {
			istag: &imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "test:tag",
				},
				Image: imageapi.Image{ObjectMeta: metav1.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
			},
		},
	}

	for name, tc := range tests {
		func() {
			client, server, storage := setup(t)
			defer server.Terminate(t)

			client.Put(
				context.TODO(),
				etcdtest.AddPrefix("/imagestreams/default/test"),
				runtime.EncodeOrDie(legacyscheme.Codecs.LegacyCodec(v1.SchemeGroupVersion),
					&imageapi.ImageStream{
						ObjectMeta: metav1.ObjectMeta{
							CreationTimestamp: metav1.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
							Namespace:         "default",
							Name:              "test",
						},
						Spec: imageapi.ImageStreamSpec{
							Tags: map[string]imageapi.TagReference{},
						},
					},
				))

			ctx := apirequest.WithUser(apirequest.NewDefaultContext(), &fakeUser{})
			_, err := storage.Create(ctx, tc.istag, rest.ValidateAllObjectFunc, false)
			gotErr := err != nil
			if e, a := tc.expectError, gotErr; e != a {
				t.Errorf("%s: Expected err=%v: got %v: %v", name, e, a, err)
				return
			}
			if tc.expectError {
				status := err.(statusError).Status()
				if status.Details.Kind != tc.errorTargetKind || status.Details.Name != tc.errorTargetID {
					t.Errorf("%s: unexpected status: %#v", name, status.Details)
					return
				}
			}
		}()
	}
}
