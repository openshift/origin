package etcd

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	etcdtesting "k8s.io/kubernetes/pkg/storage/etcd/testing"

	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/admission/testutil"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util/restoptions"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

const (
	name = "foo"
)

var (
	testDefaultRegistry = api.DefaultRegistryFunc(func() (string, bool) { return "test", true })
	noDefaultRegistry   = api.DefaultRegistryFunc(func() (string, bool) { return "", false })
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

func newStorage(t *testing.T) (*REST, *StatusREST, *InternalREST, *etcdtesting.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, latest.Version.Group)
	imageStorage, statusStorage, internalStorage, err := NewREST(restoptions.NewSimpleGetter(etcdStorage), noDefaultRegistry, &fakeSubjectAccessReviewRegistry{}, &testutil.FakeImageStreamLimitVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	return imageStorage, statusStorage, internalStorage, server
}

func validImageStream() *api.ImageStream {
	return &api.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
	}
}

func create(t *testing.T, storage *REST, obj *api.ImageStream) *api.ImageStream {
	ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
	newObj, err := storage.Create(ctx, obj)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	return newObj.(*api.ImageStream)
}

func TestCreate(t *testing.T) {
	storage, _, _, server := newStorage(t)
	defer server.Terminate(t)

	create(t, storage, validImageStream())
}

func TestList(t *testing.T) {
	storage, _, _, server := newStorage(t)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Store)
	test.TestList(
		validImageStream(),
	)
}

func TestGetImageStreamError(t *testing.T) {
	storage, _, _, server := newStorage(t)
	defer server.Terminate(t)

	image, err := storage.Get(kapi.NewDefaultContext(), "image1")
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not-found error, got %v", err)
	}
	if image != nil {
		t.Errorf("Unexpected non-nil image stream: %#v", image)
	}
}

func TestGetImageStreamOK(t *testing.T) {
	storage, _, _, server := newStorage(t)
	defer server.Terminate(t)

	image := create(t, storage, validImageStream())

	obj, err := storage.Get(kapi.NewDefaultContext(), name)
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if obj == nil {
		t.Fatalf("Unexpected nil stream")
	}
	got := obj.(*api.ImageStream)
	got.ResourceVersion = image.ResourceVersion
	if !kapi.Semantic.DeepEqual(image, got) {
		t.Errorf("Expected %#v, got %#v", image, got)
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

func (u *fakeUser) GetExtra() map[string][]string {
	return map[string][]string{}
}
