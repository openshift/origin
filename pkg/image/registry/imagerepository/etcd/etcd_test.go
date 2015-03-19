package etcd

import (
	"fmt"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest/resttest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/coreos/go-etcd/etcd"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
)

var (
	testDefaultRegistry = imagerepository.DefaultRegistryFunc(func() (string, bool) { return "test", true })
	noDefaultRegistry   = imagerepository.DefaultRegistryFunc(func() (string, bool) { return "", false })
)

func newHelper(t *testing.T) (*tools.FakeEtcdClient, tools.EtcdHelper) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	helper := tools.NewEtcdHelper(fakeEtcdClient, latest.Codec)
	return fakeEtcdClient, helper
}

func validNewRepo() *api.ImageRepository {
	return &api.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	}
}

func TestCreate(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage, _ := NewREST(helper, noDefaultRegistry)
	test := resttest.New(t, storage, fakeEtcdClient.SetError)
	repo := validNewRepo()
	repo.ObjectMeta = kapi.ObjectMeta{}
	test.TestCreate(
		// valid
		repo,
		// invalid
		&api.ImageRepository{},
	)
}

func TestGetImageRepositoryError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("foo")
	storage, _ := NewREST(helper, noDefaultRegistry)

	image, err := storage.Get(kapi.NewDefaultContext(), "image1")
	if image != nil {
		t.Errorf("Unexpected non-nil image repository: %#v", image)
	}
	if err != fakeEtcdClient.Err {
		t.Errorf("Expected %#v, got %#v", fakeEtcdClient.Err, err)
	}
}

func TestGetImageRepositoryOK(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage, _ := NewREST(helper, noDefaultRegistry)

	ctx := kapi.NewDefaultContext()
	repoName := "foo"
	key, _ := storage.store.KeyFunc(ctx, repoName)
	fakeEtcdClient.Set(key, runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: repoName}}), 0)

	obj, err := storage.Get(kapi.NewDefaultContext(), repoName)
	if obj == nil {
		t.Fatalf("Unexpected nil repo")
	}
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	repo := obj.(*api.ImageRepository)
	if e, a := repoName, repo.Name; e != a {
		t.Errorf("Expected %#v, got %#v", e, a)
	}
}

func TestListImageRepositoriesError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("foo")
	storage, _ := NewREST(helper, noDefaultRegistry)

	imageRepositories, err := storage.List(kapi.NewDefaultContext(), nil, nil)
	if err != fakeEtcdClient.Err {
		t.Errorf("Expected %#v, Got %#v", fakeEtcdClient.Err, err)
	}

	if imageRepositories != nil {
		t.Errorf("Unexpected non-nil imageRepositories list: %#v", imageRepositories)
	}
}

func TestListImageRepositoriesEmptyList(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.ChangeIndex = 1
	fakeEtcdClient.Data["/imageRepositories/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{},
		E: fakeEtcdClient.NewError(tools.EtcdErrorCodeNotFound),
	}
	storage, _ := NewREST(helper, noDefaultRegistry)

	imageRepositories, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
	if len(imageRepositories.(*api.ImageRepositoryList).Items) != 0 {
		t.Errorf("Unexpected non-zero imageRepositories list: %#v", imageRepositories)
	}
	if imageRepositories.(*api.ImageRepositoryList).ResourceVersion != "1" {
		t.Errorf("Unexpected resource version: %#v", imageRepositories)
	}
}

func TestListImageRepositoriesPopulatedList(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage, _ := NewREST(helper, noDefaultRegistry)

	fakeEtcdClient.Data["/imageRepositories/default"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})},
					{Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "bar"}})},
				},
			},
		},
	}

	list, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}

	imageRepositories := list.(*api.ImageRepositoryList)

	if e, a := 2, len(imageRepositories.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestCreateImageRepositoryOK(t *testing.T) {
	_, helper := newHelper(t)
	storage, _ := NewREST(helper, noDefaultRegistry)

	repo := &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}
	_, err := storage.Create(kapi.NewDefaultContext(), repo)
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
	if repo.CreationTimestamp.IsZero() {
		t.Error("Unexpected zero CreationTimestamp")
	}
	if repo.DockerImageRepository != "" {
		t.Errorf("unexpected repository: %#v", repo)
	}
}

func TestCreateRegistryErrorSaving(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("foo")
	storage, _ := NewREST(helper, noDefaultRegistry)

	_, err := storage.Create(kapi.NewDefaultContext(), &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})
	if err != fakeEtcdClient.Err {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
}

func TestUpdateImageRepositoryMissingID(t *testing.T) {
	_, helper := newHelper(t)
	storage, _ := NewREST(helper, noDefaultRegistry)

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.ImageRepository{})
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
	storage, _ := NewREST(helper, noDefaultRegistry)

	_, created, err := storage.Update(kapi.NewDefaultContext(), &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "bar"}})
	if err != fakeEtcdClient.Err || created {
		t.Fatalf("Unexpected non-nil error: %#v", err)
	}
}

func TestUpdateImageRepositoryOK(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Data["/imageRepositories/default/bar"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{
					ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "default"},
				}),
				ModifiedIndex: 2,
			},
		},
	}
	storage, _ := NewREST(helper, noDefaultRegistry)

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "bar", ResourceVersion: "1"}})
	if !errors.IsConflict(err) {
		t.Fatalf("unexpected non-error: %v", err)
	}
	obj, created, err = storage.Update(kapi.NewDefaultContext(), &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "bar", ResourceVersion: "2"}})
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
	fakeEtcdClient, helper := newHelper(t)
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
	storage, _ := NewREST(helper, noDefaultRegistry)

	obj, err := storage.Delete(kapi.NewDefaultContext(), "foo")
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
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Data["/imageRepositories/legal-name/bar"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{
					ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "default"},
				}),
				ModifiedIndex: 2,
			},
		},
	}
	storage, _ := NewREST(helper, noDefaultRegistry)

	obj, created, err := storage.Update(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.ImageRepository{
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
func TestEtcdListImagesRepositoriesEmpty(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageRepositoriesListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	repos, err := registry.ListImageRepositories(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(repos.Items) != 0 {
		t.Errorf("Unexpected image repositories list: %#v", repos)
	}
}

func TestEtcdListImageRepositoriesError(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageRepositoriesListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	repos, err := registry.ListImageRepositories(kapi.NewDefaultContext(), labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if repos != nil {
		t.Errorf("Unexpected non-nil repos: %#v", repos)
	}
}

func TestEtcdListImageRepositoriesEverything(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageRepositoriesListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	registry.defaultRegistry = testDefaultRegistry
	repos, err := registry.ListImageRepositories(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(repos.Items) != 2 || repos.Items[0].Name != "foo" || repos.Items[1].Name != "bar" || repos.Items[1].Status.DockerImageRepository != "test/default/bar" {
		t.Errorf("Unexpected images list: %#v", repos)
	}
}

func TestEtcdListImageRepositoriesFiltered(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageRepositoriesListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{
							ObjectMeta: kapi.ObjectMeta{
								Name:   "foo",
								Labels: map[string]string{"env": "prod"},
							},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{
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
	repos, err := registry.ListImageRepositories(kapi.NewDefaultContext(), labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(repos.Items) != 1 || repos.Items[0].Name != "bar" {
		t.Errorf("Unexpected repos list: %#v", repos)
	}
}

func TestEtcdGetImageRepository(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(makeTestDefaultImageRepositoriesKey("foo"), runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	repo, err := registry.GetImageRepository(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if repo.Name != "foo" {
		t.Errorf("Unexpected repo: %#v", repo)
	}
}

func TestEtcdGetImageRepositoryNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultImageRepositoriesKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	repo, err := registry.GetImageRepository(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
	if repo != nil {
		t.Errorf("Unexpected non-nil repo: %#v", repo)
	}
}

func TestEtcdCreateImageRepository(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data[makeTestDefaultImageRepositoriesKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImageRepository(kapi.NewDefaultContext(), &api.ImageRepository{
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

	resp, err := fakeClient.Get(makeTestDefaultImageRepositoriesKey("foo"), false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var repo api.ImageRepository
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &repo)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if repo.Name != "foo" {
		t.Errorf("Unexpected repo: %#v %s", repo, resp.Node.Value)
	}

	if len(repo.Labels) != 1 || repo.Labels["a"] != "b" {
		t.Errorf("Unexpected labels: %#v", repo.Labels)
	}

	if repo.DockerImageRepository != "c/d" {
		t.Errorf("Unexpected docker image repo: %s", repo.DockerImageRepository)
	}

	if len(repo.Tags) != 1 || repo.Tags["t1"] != "v1" {
		t.Errorf("Unexpected tags: %#v", repo.Tags)
	}
}

func TestEtcdCreateImageRepositoryAlreadyExists(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultImageRepositoriesKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImageRepository(kapi.NewDefaultContext(), &api.ImageRepository{
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

func TestEtcdUpdateImageRepository(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true

	resp, _ := fakeClient.Set(makeTestDefaultImageRepositoriesKey("foo"), runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	err := registry.UpdateImageRepository(kapi.NewDefaultContext(), &api.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "foo", ResourceVersion: strconv.FormatUint(resp.Node.ModifiedIndex, 10)},
		DockerImageRepository: "some/repo",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	repo, err := registry.GetImageRepository(kapi.NewDefaultContext(), "foo")
	if repo.DockerImageRepository != "some/repo" {
		t.Errorf("Unexpected repo: %#v", repo)
	}
}

func TestEtcdDeleteImageRepositoryNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteImageRepository(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("Expected 'not found' error, got %#v", err)
	}
}

func TestEtcdDeleteImageRepositoryError(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = fmt.Errorf("Some error")
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteImageRepository(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteImageRepositoryOK(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	key := makeTestDefaultImageRepositoriesListKey() + "/foo"
	err := registry.DeleteImageRepository(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}

func TestEtcdWatchImageRepositories(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)

	var tests = []struct {
		label    labels.Selector
		field    labels.Selector
		repos    []*api.ImageRepository
		expected []bool
	}{
		// want everything
		{
			labels.Everything(),
			labels.Everything(),
			[]*api.ImageRepository{
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
			[]*api.ImageRepository{
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
			[]*api.ImageRepository{
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
		// want name=foo, label color:blue, dockerImageRepository=r1
		{
			labels.SelectorFromSet(labels.Set{"color": "blue"}),
			labels.SelectorFromSet(labels.Set{"dockerImageRepository": "r1", "name": "foo"}),
			[]*api.ImageRepository{
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
		watching, err := registry.WatchImageRepositories(kapi.NewDefaultContext(), tt.label, tt.field, "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fakeClient.WaitForWatchCompletion()

		for testIndex, repo := range tt.repos {
			// Set this value to avoid duplication in tests
			repo.Status.DockerImageRepository = repo.DockerImageRepository
			repoBytes, _ := latest.Codec.Encode(repo)
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
					t.Errorf("unexpected imageRepository returned from watch: %#v", event.Object)
				}
				if e, a := watch.Added, event.Type; e != a {
					t.Errorf("Expected %v, got %v", e, a)
				}
				if e, a := repo, event.Object; !reflect.DeepEqual(e, a) {
					t.Errorf("Expected %#v, got %#v", e, a)
				}
			case <-time.After(50 * time.Millisecond):
				if tt.expected[testIndex] {
					t.Errorf("Expected imageRepository %#v to be returned from watch", repo)
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

func TestEtcdCreateImageRepositoryFailsWithoutNamespace(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImageRepository(kapi.NewContext(), &api.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})

	if err == nil {
		t.Errorf("expected error that namespace was missing from context")
	}
}

func TestEtcdListImageRepositoriesInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Data["/imageRepositories/alfa"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo1"}}),
					},
				},
			},
		},
		E: nil,
	}
	fakeClient.Data["/imageRepositories/bravo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo2"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "bar2"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)

	imageRepositoriesAlfa, err := registry.ListImageRepositories(namespaceAlfa, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(imageRepositoriesAlfa.Items) != 1 || imageRepositoriesAlfa.Items[0].Name != "foo1" {
		t.Errorf("Unexpected imageRepository list: %#v", imageRepositoriesAlfa)
	}

	imageRepositoriesBravo, err := registry.ListImageRepositories(namespaceBravo, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(imageRepositoriesBravo.Items) != 2 || imageRepositoriesBravo.Items[0].Name != "foo2" || imageRepositoriesBravo.Items[1].Name != "bar2" {
		t.Errorf("Unexpected imageRepository list: %#v", imageRepositoriesBravo)
	}
}

func TestEtcdGetImageRepositoryInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Set("/imageRepositories/alfa/foo", runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	fakeClient.Set("/imageRepositories/bravo/foo", runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)

	alfaFoo, err := registry.GetImageRepository(namespaceAlfa, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if alfaFoo == nil || alfaFoo.Name != "foo" {
		t.Errorf("Unexpected deployment: %#v", alfaFoo)
	}

	bravoFoo, err := registry.GetImageRepository(namespaceBravo, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if bravoFoo == nil || bravoFoo.Name != "foo" {
		t.Errorf("Unexpected deployment: %#v", bravoFoo)
	}
}
*/
