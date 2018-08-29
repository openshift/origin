package etcd

import (
	"testing"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation/fake"
	admfake "github.com/openshift/origin/pkg/image/apiserver/admission/fake"
	authorizationapi "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistrytest "k8s.io/apiserver/pkg/registry/generic/testing"
	"k8s.io/apiserver/pkg/registry/rest"
	etcdtesting "k8s.io/apiserver/pkg/storage/etcd/testing"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
)

const (
	name = "foo"
)

var (
	testDefaultRegistry = func() (string, bool) { return "test", true }
	noDefaultRegistry   = func() (string, bool) { return "", false }
)

type fakeSubjectAccessReviewRegistry struct {
	err              error
	allow            bool
	request          *authorizationapi.SubjectAccessReview
	requestNamespace string
}

func (f *fakeSubjectAccessReviewRegistry) Create(subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReview, error) {
	f.request = subjectAccessReview
	f.requestNamespace = subjectAccessReview.Spec.ResourceAttributes.Namespace
	return &authorizationapi.SubjectAccessReview{
		Status: authorizationapi.SubjectAccessReviewStatus{
			Allowed: f.allow,
		},
	}, f.err
}

func newStorage(t *testing.T) (*REST, *StatusREST, *InternalREST, *etcdtesting.EtcdTestServer) {
	server, etcdStorage := etcdtesting.NewUnsecuredEtcd3TestClientServer(t)
	etcdStorage.Codec = legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Group: "image.openshift.io", Version: "v1"})
	imagestreamRESTOptions := generic.RESTOptions{StorageConfig: etcdStorage, Decorator: generic.UndecoratedStorage, DeleteCollectionWorkers: 1, ResourcePrefix: "imagestreams"}
	registry := registryhostname.TestingRegistryHostnameRetriever(noDefaultRegistry, "", "")
	imageStorage, _, statusStorage, internalStorage, err := NewREST(
		imagestreamRESTOptions,
		registry,
		&fakeSubjectAccessReviewRegistry{},
		&admfake.ImageStreamLimitVerifier{},
		&fake.RegistryWhitelister{},
		NewEmptyLayerIndex(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return imageStorage, statusStorage, internalStorage, server
}

func validImageStream() *imageapi.ImageStream {
	return &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func create(t *testing.T, storage *REST, obj *imageapi.ImageStream) *imageapi.ImageStream {
	ctx := apirequest.WithUser(apirequest.NewDefaultContext(), &fakeUser{})
	newObj, err := storage.Create(ctx, obj, rest.ValidateAllObjectFunc, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	return newObj.(*imageapi.ImageStream)
}

func TestCreate(t *testing.T) {
	storage, _, _, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()

	// TODO switch to upstream testing suite, when there will be possibility
	// to inject context with user, needed for these tests
	create(t, storage, validImageStream())
}

func TestList(t *testing.T) {
	storage, _, _, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := genericregistrytest.New(t, storage.Store)
	test.TestList(
		validImageStream(),
	)
}

func TestGetImageStreamError(t *testing.T) {
	storage, _, _, server := newStorage(t)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()

	image, err := storage.Get(apirequest.NewDefaultContext(), "image1", &metav1.GetOptions{})
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
	defer storage.Store.DestroyFunc()

	image := create(t, storage, validImageStream())

	obj, err := storage.Get(apirequest.NewDefaultContext(), name, &metav1.GetOptions{})
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if obj == nil {
		t.Fatalf("Unexpected nil stream")
	}
	got := obj.(*imageapi.ImageStream)
	got.ResourceVersion = image.ResourceVersion
	if !kapihelper.Semantic.DeepEqual(image, got) {
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
