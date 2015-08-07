package etcd

import (
	"fmt"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/storage"
	etcdstorage "github.com/GoogleCloudPlatform/kubernetes/pkg/storage/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools/etcdtest"
	"github.com/coreos/go-etcd/etcd"
	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
)

var (
	testDefaultRegistry = imagestream.DefaultRegistryFunc(func() (string, bool) { return "test", true })
	noDefaultRegistry   = imagestream.DefaultRegistryFunc(func() (string, bool) { return "", false })
)

type fakeSubjectAccessReviewRegistry struct {
	err              error
	allow            bool
	request          *authorizationapi.SubjectAccessReview
	requestNamespace string
}

var _ subjectaccessreview.Registry = &fakeSubjectAccessReviewRegistry{}

func (f *fakeSubjectAccessReviewRegistry) CreateSubjectAccessReview(ctx kapi.Context, subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	f.request = subjectAccessReview
	f.requestNamespace = kapi.NamespaceValue(ctx)
	return &authorizationapi.SubjectAccessReviewResponse{Allowed: f.allow}, f.err
}

func newHelper(t *testing.T) (*tools.FakeEtcdClient, storage.Interface) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	helper := etcdstorage.NewEtcdStorage(fakeEtcdClient, latest.Codec, etcdtest.PathPrefix())
	return fakeEtcdClient, helper
}

func validNewStream() *api.ImageStream {
	return &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	}
}

func TestCreate(t *testing.T) {
	_, helper := newHelper(t)
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})
	stream := validNewStream()
	ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
	_, err := storage.Create(ctx, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetImageStreamError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("foo")
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	image, err := storage.Get(kapi.NewDefaultContext(), "image1")
	if image != nil {
		t.Errorf("Unexpected non-nil image stream: %#v", image)
	}
	if err != fakeEtcdClient.Err {
		t.Errorf("Expected %#v, got %#v", fakeEtcdClient.Err, err)
	}
}

func TestGetImageStreamOK(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	ctx := kapi.NewDefaultContext()
	repoName := "foo"
	key, _ := storage.store.KeyFunc(ctx, repoName)
	fakeEtcdClient.Set(key, runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: repoName}}), 0)

	obj, err := storage.Get(kapi.NewDefaultContext(), repoName)
	if obj == nil {
		t.Fatalf("Unexpected nil stream")
	}
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	stream := obj.(*api.ImageStream)
	if e, a := repoName, stream.Name; e != a {
		t.Errorf("Expected %#v, got %#v", e, a)
	}
}

func TestListImageStreamsError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("foo")
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	imageStreams, err := storage.List(kapi.NewDefaultContext(), nil, nil)
	if err != fakeEtcdClient.Err {
		t.Errorf("Expected %#v, Got %#v", fakeEtcdClient.Err, err)
	}

	if imageStreams != nil {
		t.Errorf("Unexpected non-nil imageStreams list: %#v", imageStreams)
	}
}

func TestListImageStreamsEmptyList(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.ChangeIndex = 1
	fakeEtcdClient.Data["/imagestreams/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{},
		E: fakeEtcdClient.NewError(tools.EtcdErrorCodeNotFound),
	}
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	imageStreams, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	if len(imageStreams.(*api.ImageStreamList).Items) != 0 {
		t.Errorf("Unexpected non-zero imageStreams list: %#v", imageStreams)
	}
	if imageStreams.(*api.ImageStreamList).ResourceVersion != "1" {
		t.Errorf("Unexpected resource version: %#v", imageStreams)
	}
}

func TestListImageStreamsPopulatedList(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	fakeEtcdClient.Data["/imagestreams/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})},
					{Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "bar"}})},
				},
			},
		},
	}

	list, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}

	imageStreams := list.(*api.ImageStreamList)

	if e, a := 2, len(imageStreams.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
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

func TestCreateImageStreamOK(t *testing.T) {
	_, helper := newHelper(t)
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	stream := &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}
	ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
	_, err := storage.Create(ctx, stream)
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}

	actual := &api.ImageStream{}
	if err := helper.Get("/imagestreams/default/foo", actual, false); err != nil {
		t.Fatalf("unexpected extraction error: %v", err)
	}
	if actual.Name != stream.Name {
		t.Errorf("unexpected stream: %#v", actual)
	}
	if len(actual.UID) == 0 {
		t.Errorf("expected stream UID to be set: %#v", actual)
	}
	if stream.CreationTimestamp.IsZero() {
		t.Error("Unexpected zero CreationTimestamp")
	}
	if stream.Spec.DockerImageRepository != "" {
		t.Errorf("unexpected stream: %#v", stream)
	}
}

func TestCreateImageStreamSpecTagsFromSet(t *testing.T) {
	tests := map[string]struct {
		otherNamespace string
		sarExpected    bool
		sarAllowed     bool
	}{
		"same namespace (blank), no sar": {
			otherNamespace: "",
			sarExpected:    false,
		},
		"same namespace (set), no sar": {
			otherNamespace: "default",
			sarExpected:    false,
		},
		"different namespace, sar allowed": {
			otherNamespace: "otherns",
			sarExpected:    true,
			sarAllowed:     true,
		},
		"different namespace, sar denied": {
			otherNamespace: "otherns",
			sarExpected:    true,
			sarAllowed:     false,
		},
	}
	for name, test := range tests {
		fakeEtcdClient, helper := newHelper(t)
		sarRegistry := &fakeSubjectAccessReviewRegistry{
			allow: test.sarAllowed,
		}
		storage, _ := NewREST(helper, noDefaultRegistry, sarRegistry)

		otherNamespace := test.otherNamespace
		if len(otherNamespace) == 0 {
			otherNamespace = "default"
		}
		fakeEtcdClient.Data[fmt.Sprintf("/imagestreams/%s/other", otherNamespace)] = tools.EtcdResponseWithError{
			R: &etcd.Response{
				Node: &etcd.Node{
					Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{
						ObjectMeta: kapi.ObjectMeta{Name: "other", Namespace: otherNamespace},
						Status: api.ImageStreamStatus{
							Tags: map[string]api.TagEventList{
								"latest": {
									Items: []api.TagEvent{
										{
											DockerImageReference: fmt.Sprintf("%s/other:latest", otherNamespace),
										},
									},
								},
							},
						},
					}),
					ModifiedIndex: 1,
				},
			},
		}

		stream := &api.ImageStream{
			ObjectMeta: kapi.ObjectMeta{Name: "foo"},
			Spec: api.ImageStreamSpec{
				Tags: map[string]api.TagReference{
					"other": {
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Namespace: test.otherNamespace,
							Name:      "other:latest",
						},
					},
				},
			},
		}
		ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
		_, err := storage.Create(ctx, stream)
		if test.sarExpected {
			if sarRegistry.request == nil {
				t.Errorf("%s: expected sar request", name)
				continue
			}
			if e, a := test.sarAllowed, err == nil; e != a {
				t.Errorf("%s: expected sarAllowed=%t, got error %t: %v", name, e, a, err)
				continue
			}

			continue
		}

		// sar not expected
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}

		actual := &api.ImageStream{}
		if err := helper.Get("/imagestreams/default/foo", actual, false); err != nil {
			t.Fatalf("%s: unexpected extraction error: %v", name, err)
		}
		if e, a := fmt.Sprintf("%s/other:latest", otherNamespace), actual.Status.Tags["other"].Items[0].DockerImageReference; e != a {
			t.Errorf("%s: dockerImageReference: expected %q, got %q", name, e, a)
		}
	}
}

func TestCreateRegistryErrorSaving(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("foo")
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
	_, err := storage.Create(ctx, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})
	if err != fakeEtcdClient.Err {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
}

func TestUpdateImageStreamMissingID(t *testing.T) {
	_, helper := newHelper(t)
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.ImageStream{})
	if obj != nil || created {
		t.Fatalf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "Name parameter required") == -1 {
		t.Errorf("Expected 'Name parameter required' error, got %v", err)
	}
}

func TestUpdateRegistryErrorSaving(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("foo")
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	_, created, err := storage.Update(kapi.NewDefaultContext(), &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "bar"}})
	if err != fakeEtcdClient.Err || created {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
}

func TestUpdateImageStreamOK(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Data["/imagestreams/default/bar"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{
					ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "default"},
				}),
				ModifiedIndex: 2,
			},
		},
	}
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
	obj, created, err := storage.Update(ctx, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "bar", ResourceVersion: "1"}})
	if !errors.IsConflict(err) {
		t.Fatalf("unexpected non-error: %v", err)
	}
	obj, created, err = storage.Update(ctx, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "bar", ResourceVersion: "2"}})
	if err != nil || created {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	stream, ok := obj.(*api.ImageStream)
	if !ok {
		t.Errorf("Expected image stream, got %#v", obj)
	}
	if stream.Name != "bar" {
		t.Errorf("Unexpected stream returned: %#v", stream)
	}
}

func TestUpdateImageStreamSpecTagsFromSet(t *testing.T) {
	tests := map[string]struct {
		otherNamespace string
		sarExpected    bool
		sarAllowed     bool
	}{
		"same namespace (blank), no sar": {
			otherNamespace: "",
			sarExpected:    false,
		},
		"same namespace (set), no sar": {
			otherNamespace: "default",
			sarExpected:    false,
		},
		"different namespace, sar allowed": {
			otherNamespace: "otherns",
			sarExpected:    true,
			sarAllowed:     true,
		},
		"different namespace, sar denied": {
			otherNamespace: "otherns",
			sarExpected:    true,
			sarAllowed:     false,
		},
	}
	for name, test := range tests {
		fakeEtcdClient, helper := newHelper(t)
		sarRegistry := &fakeSubjectAccessReviewRegistry{
			allow: test.sarAllowed,
		}
		storage, _ := NewREST(helper, noDefaultRegistry, sarRegistry)

		fakeEtcdClient.Data["/imagestreams/default/foo"] = tools.EtcdResponseWithError{
			R: &etcd.Response{
				Node: &etcd.Node{
					Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{
						ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "default"},
					}),
					ModifiedIndex: 1,
				},
			},
		}

		otherNamespace := test.otherNamespace
		if len(otherNamespace) == 0 {
			otherNamespace = "default"
		}
		fakeEtcdClient.Data[fmt.Sprintf("/imagestreams/%s/other", otherNamespace)] = tools.EtcdResponseWithError{
			R: &etcd.Response{
				Node: &etcd.Node{
					Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{
						ObjectMeta: kapi.ObjectMeta{Name: "other", Namespace: otherNamespace},
						Status: api.ImageStreamStatus{
							Tags: map[string]api.TagEventList{
								"latest": {
									Items: []api.TagEvent{
										{
											DockerImageReference: fmt.Sprintf("%s/other:latest", otherNamespace),
										},
									},
								},
							},
						},
					}),
					ModifiedIndex: 1,
				},
			},
		}

		stream := &api.ImageStream{
			ObjectMeta: kapi.ObjectMeta{Name: "foo", ResourceVersion: "1"},
			Spec: api.ImageStreamSpec{
				Tags: map[string]api.TagReference{
					"other": {
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamTag",
							Namespace: test.otherNamespace,
							Name:      "other:latest",
						},
					},
				},
			},
		}
		ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
		_, _, err := storage.Update(ctx, stream)
		if test.sarExpected {
			if sarRegistry.request == nil {
				t.Errorf("%s: expected sar request", name)
				continue
			}
			if e, a := test.sarAllowed, err == nil; e != a {
				t.Errorf("%s: expected sarAllowed=%t, got error %t: %v", name, e, a, err)
				continue
			}

			continue
		}

		// sar not expected
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}

		actual := &api.ImageStream{}
		if err := helper.Get("/imagestreams/default/foo", actual, false); err != nil {
			t.Fatalf("%s: unexpected extraction error: %v", name, err)
		}
		if e, a := fmt.Sprintf("%s/other:latest", otherNamespace), actual.Status.Tags["other"].Items[0].DockerImageReference; e != a {
			t.Errorf("%s: dockerImageReference: expected %q, got %q", name, e, a)
		}
	}
}

func TestDeleteImageStream(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Data["/imagestreams/default/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{
					ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "default"},
				}),
				ModifiedIndex: 2,
			},
		},
	}
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	obj, err := storage.Delete(kapi.NewDefaultContext(), "foo", nil)
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	status, ok := obj.(*kapi.Status)
	if !ok {
		t.Fatalf("Expected status, got %#v", obj)
	}
	if status.Status != kapi.StatusSuccess {
		t.Errorf("Expected status=success, got %#v", status)
	}
}

func TestUpdateImageStreamConflictingNamespace(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Data["/imagestreams/legal-name/bar"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{
					ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "default"},
				}),
				ModifiedIndex: 2,
			},
		},
	}
	storage, _ := NewREST(helper, noDefaultRegistry, &fakeSubjectAccessReviewRegistry{})

	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &fakeUser{})
	obj, created, err := storage.Update(ctx, &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "some-value", ResourceVersion: "2"},
	})

	if obj != nil || created {
		t.Error("Expected a nil obj, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func checkExpectedNamespaceError(t *testing.T, err error) {
	expectedError := "the namespace of the provided object does not match the namespace sent on the request"
	if err == nil {
		t.Fatalf("Expected '" + expectedError + "', but we didn't get one")
	}
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected '"+expectedError+"' error, got '%v'", err.Error())
	}

}

/*
func TestEtcdListImagesStreamsEmpty(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageStreamsListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	repos, err := registry.ListImageStreams(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(repos.Items) != 0 {
		t.Errorf("Unexpected image streams list: %#v", repos)
	}
}

func TestEtcdListImageStreamsError(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageStreamsListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	repos, err := registry.ListImageStreams(kapi.NewDefaultContext(), labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if repos != nil {
		t.Errorf("Unexpected non-nil repos: %#v", repos)
	}
}

func TestEtcdListImageStreamsEverything(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageStreamsListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	registry.defaultRegistry = testDefaultRegistry
	repos, err := registry.ListImageStreams(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(repos.Items) != 2 || repos.Items[0].Name != "foo" || repos.Items[1].Name != "bar" || repos.Items[1].Status.DockerImageRepository != "test/default/bar" {
		t.Errorf("Unexpected images list: %#v", repos)
	}
}

func TestEtcdListImageStreamsFiltered(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageStreamsListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{
							ObjectMeta: kapi.ObjectMeta{
								Name:   "foo",
								Labels: map[string]string{"env": "prod"},
							},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{
							ObjectMeta: kapi.ObjectMeta{
								Name:   "bar",
								Labels: map[string]string{"env": "dev"},
							},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	repos, err := registry.ListImageStreams(kapi.NewDefaultContext(), labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(repos.Items) != 1 || repos.Items[0].Name != "bar" {
		t.Errorf("Unexpected repos list: %#v", repos)
	}
}

func TestEtcdGetImageStream(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(makeTestDefaultImageStreamsKey("foo"), runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	stream, err := registry.GetImageStream(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if stream.Name != "foo" {
		t.Errorf("Unexpected stream: %#v", stream)
	}
}

func TestEtcdGetImageStreamNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultImageStreamsKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	stream, err := registry.GetImageStream(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
	if stream != nil {
		t.Errorf("Unexpected non-nil stream: %#v", stream)
	}
}

func TestEtcdCreateImageStream(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data[makeTestDefaultImageStreamsKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImageStream(kapi.NewDefaultContext(), &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name:   "foo",
			Labels: map[string]string{"a": "b"},
		},
		DockerImageRepository: "c/d",
		Tags: map[string]string{"t1": "v1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get(makeTestDefaultImageStreamsKey("foo"), false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var stream api.ImageStream
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &stream)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if stream.Name != "foo" {
		t.Errorf("Unexpected stream: %#v %s", stream, resp.Node.Value)
	}

	if len(stream.Labels) != 1 || stream.Labels["a"] != "b" {
		t.Errorf("Unexpected labels: %#v", stream.Labels)
	}

	if stream.DockerImageRepository != "c/d" {
		t.Errorf("Unexpected docker image stream: %s", stream.DockerImageRepository)
	}

	if len(stream.Tags) != 1 || stream.Tags["t1"] != "v1" {
		t.Errorf("Unexpected tags: %#v", stream.Tags)
	}
}

func TestEtcdCreateImageStreamAlreadyExists(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultImageStreamsKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImageStream(kapi.NewDefaultContext(), &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsAlreadyExists(err) {
		t.Errorf("Expected 'already exists' error, got %#v", err)
	}
}

func TestEtcdUpdateImageStream(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true

	resp, _ := fakeClient.Set(makeTestDefaultImageStreamsKey("foo"), runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	err := registry.UpdateImageStream(kapi.NewDefaultContext(), &api.ImageStream{
		ObjectMeta:            kapi.ObjectMeta{Name: "foo", ResourceVersion: strconv.FormatUint(resp.Node.ModifiedIndex, 10)},
		DockerImageRepository: "some/stream",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	stream, err := registry.GetImageStream(kapi.NewDefaultContext(), "foo")
	if stream.DockerImageRepository != "some/stream" {
		t.Errorf("Unexpected stream: %#v", stream)
	}
}

func TestEtcdDeleteImageStreamNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteImageStream(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("Expected 'not found' error, got %#v", err)
	}
}

func TestEtcdDeleteImageStreamError(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = fmt.Errorf("Some error")
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteImageStream(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteImageStreamOK(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	key := makeTestDefaultImageStreamsListKey() + "/foo"
	err := registry.DeleteImageStream(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}

func TestEtcdWatchImageStreams(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)

	var tests = []struct {
		label    labels.Selector
		field    labels.Selector
		repos    []*api.ImageStream
		expected []bool
	}{
		// want everything
		{
			labels.Everything(),
			labels.Everything(),
			[]*api.ImageStream{
				{ObjectMeta: kapi.ObjectMeta{Name: "a", Labels: labels.Set{"l1": "v1"}}, DockerImageRepository: "r1"},
				{ObjectMeta: kapi.ObjectMeta{Name: "b", Labels: labels.Set{"l2": "v2"}}, DockerImageRepository: "r2"},
				{ObjectMeta: kapi.ObjectMeta{Name: "c", Labels: labels.Set{"l3": "v3"}}, DockerImageRepository: "r3"},
			},
			[]bool{
				true,
				true,
				true,
			},
		},
		// want name=foo
		{
			labels.Everything(),
			labels.SelectorFromSet(labels.Set{"name": "foo"}),
			[]*api.ImageStream{
				{ObjectMeta: kapi.ObjectMeta{Name: "a", Labels: labels.Set{"l1": "v1"}}, DockerImageRepository: "r1"},
				{ObjectMeta: kapi.ObjectMeta{Name: "foo", Labels: labels.Set{"l2": "v2"}}, DockerImageRepository: "r2"},
				{ObjectMeta: kapi.ObjectMeta{Name: "c", Labels: labels.Set{"l3": "v3"}}, DockerImageRepository: "r3"},
			},
			[]bool{
				false,
				true,
				false,
			},
		},
		// want label color:blue
		{
			labels.SelectorFromSet(labels.Set{"color": "blue"}),
			labels.Everything(),
			[]*api.ImageStream{
				{ObjectMeta: kapi.ObjectMeta{Name: "a", Labels: labels.Set{"color": "blue"}}, DockerImageRepository: "r1"},
				{ObjectMeta: kapi.ObjectMeta{Name: "foo", Labels: labels.Set{"l2": "v2"}}, DockerImageRepository: "r2"},
				{ObjectMeta: kapi.ObjectMeta{Name: "c", Labels: labels.Set{"color": "blue"}}, DockerImageRepository: "r3"},
			},
			[]bool{
				true,
				false,
				true,
			},
		},
		// want name=foo, label color:blue, dockerImageStream=r1
		{
			labels.SelectorFromSet(labels.Set{"color": "blue"}),
			labels.SelectorFromSet(labels.Set{"dockerImageStream": "r1", "name": "foo"}),
			[]*api.ImageStream{
				{ObjectMeta: kapi.ObjectMeta{Name: "foo", Labels: labels.Set{"color": "blue"}}, DockerImageRepository: "r1"},
				{ObjectMeta: kapi.ObjectMeta{Name: "b", Labels: labels.Set{"l2": "v2"}}, DockerImageRepository: "r2"},
				{ObjectMeta: kapi.ObjectMeta{Name: "c", Labels: labels.Set{"color": "blue"}}, DockerImageRepository: "r3"},
			},
			[]bool{
				true,
				false,
				false,
			},
		},
	}

	for _, tt := range tests {
		watching, err := registry.WatchImageStreams(kapi.NewDefaultContext(), tt.label, tt.field, "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fakeClient.WaitForWatchCompletion()

		for testIndex, stream := range tt.repos {
			// Set this value to avoid duplication in tests
			stream.Status.DockerImageRepository = stream.DockerImageRepository
			repoBytes, _ := latest.Codec.Encode(stream)
			fakeClient.WatchResponse <- &etcd.Response{
				Action: "set",
				Node: &etcd.Node{
					Value: string(repoBytes),
				},
			}

			select {
			case event, ok := <-watching.ResultChan():
				if !ok {
					t.Errorf("watching channel should be open")
				}
				if !tt.expected[testIndex] {
					t.Errorf("unexpected imageStream returned from watch: %#v", event.Object)
				}
				if e, a := watch.Added, event.Type; e != a {
					t.Errorf("Expected %v, got %v", e, a)
				}
				if e, a := stream, event.Object; !reflect.DeepEqual(e, a) {
					t.Errorf("Expected %#v, got %#v", e, a)
				}
			case <-time.After(50 * time.Millisecond):
				if tt.expected[testIndex] {
					t.Errorf("Expected imageStream %#v to be returned from watch", stream)
				}
			}
		}

		select {
		case _, ok := <-watching.ResultChan():
			if !ok {
				t.Errorf("watching channel should be open")
			}
		default:
		}

		fakeClient.WatchInjectError <- nil
		if _, ok := <-watching.ResultChan(); ok {
			t.Errorf("watching channel should be closed")
		}
		watching.Stop()
	}
}

func TestEtcdCreateImageStreamFailsWithoutNamespace(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImageStream(kapi.NewContext(), &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})

	if err == nil {
		t.Errorf("expected error that namespace was missing from context")
	}
}

func TestEtcdListImageStreamsInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Data["/imagestreams/alfa"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo1"}}),
					},
				},
			},
		},
		E: nil,
	}
	fakeClient.Data["/imagestreams/bravo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo2"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "bar2"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)

	imageStreamsAlfa, err := registry.ListImageStreams(namespaceAlfa, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(imageStreamsAlfa.Items) != 1 || imageStreamsAlfa.Items[0].Name != "foo1" {
		t.Errorf("Unexpected imageStream list: %#v", imageStreamsAlfa)
	}

	imageStreamsBravo, err := registry.ListImageStreams(namespaceBravo, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(imageStreamsBravo.Items) != 2 || imageStreamsBravo.Items[0].Name != "foo2" || imageStreamsBravo.Items[1].Name != "bar2" {
		t.Errorf("Unexpected imageStream list: %#v", imageStreamsBravo)
	}
}

func TestEtcdGetImageStreamInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Set("/imagestreams/alfa/foo", runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	fakeClient.Set("/imagestreams/bravo/foo", runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)

	alfaFoo, err := registry.GetImageStream(namespaceAlfa, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if alfaFoo == nil || alfaFoo.Name != "foo" {
		t.Errorf("Unexpected deployment: %#v", alfaFoo)
	}

	bravoFoo, err := registry.GetImageStream(namespaceBravo, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if bravoFoo == nil || bravoFoo.Name != "foo" {
		t.Errorf("Unexpected deployment: %#v", bravoFoo)
	}
}
*/
type fakeStrategy struct {
	imagestream.Strategy
}

func (fakeStrategy) PrepareForCreate(obj runtime.Object) {
	stream := obj.(*api.ImageStream)
	stream.Annotations = map[string]string{"test": "PrepareForCreate"}
}

func (fakeStrategy) PrepareForUpdate(obj, old runtime.Object) {
	stream := obj.(*api.ImageStream)
	stream.Annotations["test"] = "PrepareForUpdate"
}

func TestStrategyPrepareMethods(t *testing.T) {
	_, helper := newHelper(t)
	storage, _ := NewREST(helper, testDefaultRegistry, &fakeSubjectAccessReviewRegistry{})
	stream := validNewStream()
	strategy := fakeStrategy{imagestream.NewStrategy(testDefaultRegistry, &fakeSubjectAccessReviewRegistry{})}

	storage.store.CreateStrategy = strategy
	storage.store.UpdateStrategy = strategy

	ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
	obj, err := storage.Create(ctx, stream)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	updatedStream := obj.(*api.ImageStream)
	if updatedStream.Annotations["test"] != "PrepareForCreate" {
		t.Errorf("Expected PrepareForCreate annotation")
	}

	obj, _, err = storage.Update(ctx, updatedStream)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	updatedStream = obj.(*api.ImageStream)
	if updatedStream.Annotations["test"] != "PrepareForUpdate" {
		t.Errorf("Expected PrepareForUpdate annotation")
	}
}
