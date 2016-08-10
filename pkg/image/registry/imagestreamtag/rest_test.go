package imagestreamtag

import (
	"reflect"
	"testing"
	"time"

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

	storage := NewREST(imageRegistry, imageStreamRegistry)

	return etcdClient, server, storage
}

type statusError interface {
	Status() unversioned.Status
}

func TestGetImageStreamTag(t *testing.T) {
	tests := map[string]struct {
		image           *api.Image
		repo            *api.ImageStream
		expectError     bool
		errorTargetKind string
		errorTargetID   string
	}{
		"happy path": {
			image: &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
			repo: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							Annotations: map[string]string{
								"color": "blue",
								"size":  "large",
							},
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"latest": {
							Items: []api.TagEvent{
								{
									Created:              unversioned.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
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
			repo: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test"},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"latest": {Items: []api.TagEvent{{DockerImageReference: "test", Image: ""}}},
					},
				}},
			expectError:     true,
			errorTargetKind: "imagestreamtags",
			errorTargetID:   "test:latest",
		},
		"missing image": {
			repo: &api.ImageStream{Status: api.ImageStreamStatus{
				Tags: map[string]api.TagEventList{
					"latest": {Items: []api.TagEvent{{DockerImageReference: "test", Image: "10"}}},
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
			image: &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz"},
			repo: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "test"},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"other": {Items: []api.TagEvent{{DockerImageReference: "test", Image: "10"}}},
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
				client.Create(
					context.TODO(),
					etcdtest.AddPrefix("/images/"+testCase.image.Name),
					runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), testCase.image),
				)
			}
			if testCase.repo != nil {
				client.Create(
					context.TODO(),
					etcdtest.AddPrefix("/imagestreams/default/test"),
					runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), testCase.repo),
				)
			}

			obj, err := storage.Get(kapi.NewDefaultContext(), "test:latest")
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
				actual := obj.(*api.ImageStreamTag)
				if e, a := "default", actual.Namespace; e != a {
					t.Errorf("%s: namespace: expected %v, got %v", name, e, a)
				}
				if e, a := "test:latest", actual.Name; e != a {
					t.Errorf("%s: name: expected %v, got %v", name, e, a)
				}
				if e, a := map[string]string{"size": "large", "color": "blue"}, actual.Image.Annotations; !reflect.DeepEqual(e, a) {
					t.Errorf("%s: annotations: expected %v, got %v", name, e, a)
				}
				if e, a := unversioned.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC), actual.CreationTimestamp; !a.Equal(e) {
					t.Errorf("%s: timestamp: expected %v, got %v", name, e, a)
				}
			}
		}()
	}
}

func TestGetImageStreamTagDIR(t *testing.T) {
	expDockerImageReference := "foo/bar/baz:latest"
	image := &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "10"}, DockerImageReference: "foo/bar/baz:different"}
	repo := &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "default",
			Name:      "test",
		},
		Status: api.ImageStreamStatus{
			Tags: map[string]api.TagEventList{
				"latest": {
					Items: []api.TagEvent{
						{
							Created:              unversioned.Date(2015, 3, 24, 9, 38, 0, 0, time.UTC),
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
	client.Create(
		context.TODO(),
		etcdtest.AddPrefix("/images/"+image.Name),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), image),
	)
	client.Create(
		context.TODO(),
		etcdtest.AddPrefix("/imagestreams/default/test"),
		runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), repo),
	)
	obj, err := storage.Get(kapi.NewDefaultContext(), "test:latest")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	actual := obj.(*api.ImageStreamTag)
	if actual.Image.DockerImageReference != expDockerImageReference {
		t.Errorf("Different DockerImageReference: expected %s, got %s", expDockerImageReference, actual.Image.DockerImageReference)
	}
}

func TestDeleteImageStreamTag(t *testing.T) {
	tests := map[string]struct {
		repo        *api.ImageStream
		expectError bool
	}{
		"repo not found": {
			expectError: true,
		},
		"nil tag map": {
			repo: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
			},
			expectError: true,
		},
		"missing tag": {
			repo: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
					Name:      "test",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
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
			repo: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace:  "default",
					Name:       "test",
					Generation: 2,
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
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
				Status: api.ImageStreamStatus{
					DockerImageRepository: "registry.default.local/default/test",
					Tags: map[string]api.TagEventList{
						"another": {
							Items: []api.TagEvent{
								{
									DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Generation:           2,
								},
							},
						},
						"foo": {
							Items: []api.TagEvent{
								{
									DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Generation:           2,
								},
							},
						},
						"latest": {
							Items: []api.TagEvent{
								{
									DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
									Generation:           2,
								},
							},
						},
						"bar": {
							Items: []api.TagEvent{
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
				client.Create(
					context.TODO(),
					etcdtest.AddPrefix("/imagestreams/default/test"),
					runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(v1.SchemeGroupVersion), testCase.repo),
				)
			}

			ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
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
			expectedStatus := &unversioned.Status{Status: unversioned.StatusSuccess}
			if e, a := expectedStatus, obj; !reflect.DeepEqual(e, a) {
				t.Errorf("%s:\nexpect=%#v\nactual=%#v", name, e, a)
			}

			updatedRepo, err := storage.imageStreamRegistry.GetImageStream(kapi.NewDefaultContext(), "test")
			if err != nil {
				t.Fatalf("%s: error retrieving updated repo: %s", name, err)
			}
			three := int64(3)
			expectedStreamSpec := map[string]api.TagReference{
				"another": {
					Name: "another",
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "test:foo",
					},
					Generation: &three,
				},
			}
			expectedStreamStatus := map[string]api.TagEventList{
				"another": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
							Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
							Generation:           2,
						},
					},
				},
				"foo": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry.default.local/default/test@sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
							Image:                "sha256:381151ac5b7f775e8371e489f3479b84a4c004c90ceddb2ad80b6877215a892f",
							Generation:           2,
						},
					},
				},
				"bar": {
					Items: []api.TagEvent{
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
