package imagerepository

import (
	"fmt"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/coreos/go-etcd/etcd"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
)

var testDefaultRegistry = imagestream.DefaultRegistryFunc(func() (string, bool) { return "", false })

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

func newTestHelpers(t *testing.T) (*tools.FakeEtcdClient, tools.EtcdHelper, *REST, *StatusREST) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	helper := tools.NewEtcdHelper(fakeEtcdClient, latest.Codec)
	imageStreamStorage, imageStreamStatusStorage := imagestreametcd.NewREST(helper, testDefaultRegistry, &fakeSubjectAccessReviewRegistry{})
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatusStorage)
	storage, statusStorage := NewREST(imageStreamRegistry)
	return fakeEtcdClient, helper, storage, statusStorage
}

func TestListImageRepositories(t *testing.T) {
	fakeEtcdClient, _, rest, _ := newTestHelpers(t)

	fakeEtcdClient.ChangeIndex = 1
	fakeEtcdClient.Data["/imageRepositories/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{},
		E: fakeEtcdClient.NewError(tools.EtcdErrorCodeNotFound),
	}

	obj, err := rest.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obj.(*api.ImageRepositoryList).Items) != 0 {
		t.Errorf("Unexpected non-empty list: %#v", obj)
	}
	if obj.(*api.ImageRepositoryList).ResourceVersion != "1" {
		t.Errorf("Unexpected resource version: %#v", obj)
	}
}

func TestListImageRepositoriesError(t *testing.T) {
	fakeEtcdClient, _, rest, _ := newTestHelpers(t)

	fakeEtcdClient.Err = fmt.Errorf("foo")

	obj, err := rest.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != fakeEtcdClient.Err {
		t.Errorf("Expected %v, Got %v", fakeEtcdClient.Err, err)
	}
	if obj != nil {
		t.Errorf("Unexpected non-nil list: %#v", obj)
	}
}

func TestListImageRepositoriesPopulatedList(t *testing.T) {
	fakeEtcdClient, _, rest, _ := newTestHelpers(t)

	fakeEtcdClient.Data["/imageRepositories/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})},
					{Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "bar"}})},
				},
			},
		},
	}

	obj, err := rest.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	list := obj.(*api.ImageRepositoryList)
	expected := api.ImageRepositoryList{
		Items: []api.ImageRepository{
			{ObjectMeta: kapi.ObjectMeta{Name: "foo"}},
			{ObjectMeta: kapi.ObjectMeta{Name: "bar"}},
		},
	}
	if !kapi.Semantic.DeepEqual(list, &expected) {
		t.Errorf("unexpected list - diff: %s", util.ObjectDiff(expected, list))
	}
}

func TestGetImageStreamError(t *testing.T) {
	fakeEtcdClient, _, rest, _ := newTestHelpers(t)
	fakeEtcdClient.Err = fmt.Errorf("foo")

	obj, err := rest.Get(kapi.NewDefaultContext(), "image1")
	if err != fakeEtcdClient.Err {
		t.Errorf("Expected %#v, got %#v", fakeEtcdClient.Err, err)
	}
	if obj != nil {
		t.Errorf("Unexpected non-nil obj: %#v", obj)
	}
}

func TestGetImageRepositoryOK(t *testing.T) {
	fakeEtcdClient, _, rest, _ := newTestHelpers(t)

	streamName := "foo"
	fakeEtcdClient.Set("/imageRepositories/default/foo", runtime.EncodeOrDie(latest.Codec, &api.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: streamName}}), 0)

	obj, err := rest.Get(kapi.NewDefaultContext(), streamName)
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	if obj == nil {
		t.Fatalf("Unexpected nil stream")
	}
	repo := obj.(*api.ImageRepository)
	if e, a := streamName, repo.Name; e != a {
		t.Errorf("Expected %#v, got %#v", e, a)
	}
}

func TestCreateImageRepositoryOK(t *testing.T) {
	_, helper, rest, _ := newTestHelpers(t)

	repo := &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}
	ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
	_, err := rest.Create(ctx, repo)
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}

	actual := &api.ImageRepository{}
	if err := helper.ExtractObj("/imageRepositories/default/foo", actual, false); err != nil {
		t.Fatalf("unexpected extraction error: %v", err)
	}
	if actual.Name != repo.Name {
		t.Errorf("unexpected repo: %#v", actual)
	}
	if len(actual.UID) == 0 {
		t.Errorf("expected repo UID to be set: %#v", actual)
	}
	if actual.CreationTimestamp.IsZero() {
		t.Error("Unexpected zero CreationTimestamp")
	}
	if actual.DockerImageRepository != "" {
		t.Errorf("unexpected DockerImageRepository: %#v", repo)
	}
}

func TestCreateRegistryErrorSaving(t *testing.T) {
	fakeEtcdClient, _, rest, _ := newTestHelpers(t)
	fakeEtcdClient.Err = fmt.Errorf("foo")

	ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
	_, err := rest.Create(ctx, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})
	if err != fakeEtcdClient.Err {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
}

func TestUpdateImageRepositoryMissingID(t *testing.T) {
	_, _, rest, _ := newTestHelpers(t)
	obj, created, err := rest.Update(kapi.NewDefaultContext(), &api.ImageRepository{})
	if obj != nil || created {
		t.Fatalf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "Name parameter required") == -1 {
		t.Errorf("Expected 'Name parameter required' error, got %v", err)
	}
}

func TestUpdateImageRepositoryErrorSaving(t *testing.T) {
	fakeEtcdClient, _, rest, _ := newTestHelpers(t)
	fakeEtcdClient.Err = fmt.Errorf("foo")

	_, created, err := rest.Update(kapi.NewDefaultContext(), &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "bar"}})
	if err != fakeEtcdClient.Err || created {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
}

func TestUpdateImageRepositoryOK(t *testing.T) {
	fakeEtcdClient, _, rest, _ := newTestHelpers(t)
	fakeEtcdClient.Data["/imageRepositories/default/bar"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{
					ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "default"},
				}),
				ModifiedIndex: 2,
			},
		},
	}

	ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
	obj, created, err := rest.Update(ctx, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "bar", ResourceVersion: "1"}})
	if !errors.IsConflict(err) {
		t.Fatalf("unexpected non-error: %v", err)
	}
	obj, created, err = rest.Update(ctx, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "bar", ResourceVersion: "2"}})
	if err != nil || created {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	repo, ok := obj.(*api.ImageRepository)
	if !ok {
		t.Errorf("Expected image repository, got %#v", obj)
	}
	if repo.Name != "bar" {
		t.Errorf("Unexpected repo returned: %#v", repo)
	}
}

func TestDeleteImageRepository(t *testing.T) {
	fakeEtcdClient, _, rest, _ := newTestHelpers(t)
	fakeEtcdClient.Data["/imageRepositories/default/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{
					ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "default"},
				}),
				ModifiedIndex: 2,
			},
		},
	}

	obj, err := rest.Delete(kapi.NewDefaultContext(), "foo", nil)
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

func TestUpdateImageRepositoryConflictingNamespace(t *testing.T) {
	fakeEtcdClient, _, rest, _ := newTestHelpers(t)
	fakeEtcdClient.Data["/imageRepositories/legal-name/bar"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageStream{
					ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "default"},
				}),
				ModifiedIndex: 2,
			},
		},
	}

	ctx := kapi.WithUser(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &fakeUser{})
	obj, created, err := rest.Update(ctx, &api.ImageRepository{
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
