package etcd

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/coreos/go-etcd/etcd"
	"github.com/fsouza/go-dockerclient"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/image/api"
)

// This copy and paste is not pure ignorance.  This is that we can be sure that the key is getting made as we
// expect it to. If someone changes the location of these resources by say moving all the resources to
// "/origin/resources" (which is a really good idea), then they've made a breaking change and something should
// fail to let them know they've change some significant change and that other dependent pieces may break.
func makeTestImageListKey(namespace string) string {
	if len(namespace) != 0 {
		return "/images/" + namespace
	}
	return "/images"
}
func makeTestImageKey(namespace, id string) string {
	return "/images/" + namespace + "/" + id
}
func makeTestDefaultImageKey(id string) string {
	return makeTestImageKey(kapi.NamespaceDefault, id)
}
func makeTestDefaultImageListKey() string {
	return makeTestImageListKey(kapi.NamespaceDefault)
}
func makeTestImageRepositoriesListKey(namespace string) string {
	if len(namespace) != 0 {
		return "/imageRepositories/" + namespace
	}
	return "/imageRepositories"
}
func makeTestImageRepositoriesKey(namespace, id string) string {
	return "/imageRepositories/" + namespace + "/" + id
}
func makeTestDefaultImageRepositoriesKey(id string) string {
	return makeTestImageRepositoriesKey(kapi.NamespaceDefault, id)
}
func makeTestDefaultImageRepositoriesListKey() string {
	return makeTestImageRepositoriesListKey(kapi.NamespaceDefault)
}

func NewTestEtcd(client tools.EtcdClient) *Etcd {
	return New(tools.EtcdHelper{client, latest.Codec, tools.RuntimeVersionAdapter{latest.ResourceVersioner}})
}

func TestEtcdListImagesEmpty(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	images, err := registry.ListImages(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(images.Items) != 0 {
		t.Errorf("Unexpected images list: %#v", images)
	}
}

func TestEtcdListImagesError(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	images, err := registry.ListImages(kapi.NewDefaultContext(), labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if images != nil {
		t.Errorf("Unexpected non-nil images: %#v", images)
	}
}

func TestEtcdListImagesEverything(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	images, err := registry.ListImages(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(images.Items) != 2 || images.Items[0].Name != "foo" || images.Items[1].Name != "bar" {
		t.Errorf("Unexpected images list: %#v", images)
	}
}

func TestEtcdListImagesFiltered(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := makeTestDefaultImageListKey()
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{
							ObjectMeta: kapi.ObjectMeta{
								Name:   "foo",
								Labels: map[string]string{"env": "prod"},
							},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{
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
	images, err := registry.ListImages(kapi.NewDefaultContext(), labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(images.Items) != 1 || images.Items[0].Name != "bar" {
		t.Errorf("Unexpected images list: %#v", images)
	}
}

func TestEtcdGetImage(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set(makeTestDefaultImageKey("foo"), runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	image, err := registry.GetImage(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if image.Name != "foo" {
		t.Errorf("Unexpected image: %#v", image)
	}
}

func TestEtcdGetImageNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultImageKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	image, err := registry.GetImage(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
	if image != nil {
		t.Errorf("Unexpected image: %#v", image)
	}
}

func TestEtcdCreateImage(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	fakeClient.Data[makeTestDefaultImageKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImage(kapi.NewDefaultContext(), &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
		DockerImageReference: "openshift/ruby-19-centos",
		Metadata: docker.Image{
			ID: "abc123",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get(makeTestDefaultImageKey("foo"), false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var image api.Image
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &image)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if image.Name != "foo" {
		t.Errorf("Unexpected image: %#v %s", image, resp.Node.Value)
	}

	if e, a := "openshift/ruby-19-centos", image.DockerImageReference; e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}

	if e, a := "abc123", image.Metadata.ID; e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestEtcdCreateImageAlreadyExists(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data[makeTestDefaultImageKey("foo")] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImage(kapi.NewDefaultContext(), &api.Image{
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

func TestEtcdUpdateImage(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	err := registry.UpdateImage(kapi.NewDefaultContext(), &api.Image{})
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteImageNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteImage(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("Expected 'not found' error, got %#v", err)
	}
}

func TestEtcdDeleteImageError(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = fmt.Errorf("Some error")
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteImage(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteImageOK(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	key := makeTestDefaultImageListKey() + "/foo"
	err := registry.DeleteImage(kapi.NewDefaultContext(), "foo")
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}

func TestEtcdWatchImagesErrorWithFieldSet(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)

	_, err := registry.WatchImages(kapi.NewDefaultContext(), labels.Everything(), labels.SelectorFromSet(labels.Set{"foo": "bar"}), "1")
	if err == nil {
		t.Fatal("unexpected nil error")
	}
	if err.Error() != "field selectors are not supported on images" {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestEtcdWatchImagesOK(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)

	var tests = []struct {
		label    labels.Selector
		images   []*api.Image
		expected []bool
	}{
		{
			labels.Everything(),
			[]*api.Image{
				{ObjectMeta: kapi.ObjectMeta{Name: "a"}},
				{ObjectMeta: kapi.ObjectMeta{Name: "b"}},
				{ObjectMeta: kapi.ObjectMeta{Name: "c"}},
			},
			[]bool{
				true,
				true,
				true,
			},
		},
		{
			labels.SelectorFromSet(labels.Set{"color": "blue"}),
			[]*api.Image{
				{ObjectMeta: kapi.ObjectMeta{Name: "a", Labels: map[string]string{"color": "blue"}}},
				{ObjectMeta: kapi.ObjectMeta{Name: "b", Labels: map[string]string{"color": "green"}}},
				{ObjectMeta: kapi.ObjectMeta{Name: "c", Labels: map[string]string{"color": "blue"}}},
			},
			[]bool{
				true,
				false,
				true,
			},
		},
	}
	for _, tt := range tests {
		watching, err := registry.WatchImages(kapi.NewDefaultContext(), tt.label, labels.Everything(), "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fakeClient.WaitForWatchCompletion()

		for testIndex, image := range tt.images {
			imageBytes, _ := latest.Codec.Encode(image)
			fakeClient.WatchResponse <- &etcd.Response{
				Action: "set",
				Node: &etcd.Node{
					Value: string(imageBytes),
				},
			}

			select {
			case event, ok := <-watching.ResultChan():
				if !ok {
					t.Errorf("watching channel should be open")
				}
				if !tt.expected[testIndex] {
					t.Errorf("unexpected image returned from watch: %#v", event.Object)
				}
				if e, a := watch.Added, event.Type; e != a {
					t.Errorf("Expected %v, got %v", e, a)
				}
				if e, a := image, event.Object; !reflect.DeepEqual(e, a) {
					t.Errorf("Expected %v, got %v", e, a)
				}
			case <-time.After(50 * time.Millisecond):
				if tt.expected[testIndex] {
					t.Errorf("Expected image %#v to be returned from watch", image)
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
	repos, err := registry.ListImageRepositories(kapi.NewDefaultContext(), labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(repos.Items) != 2 || repos.Items[0].Name != "foo" || repos.Items[1].Name != "bar" {
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
	filterFields := labels.SelectorFromSet(labels.Set{"name": "foo"})

	watching, err := registry.WatchImageRepositories(kapi.NewDefaultContext(), "1", func(repo *api.ImageRepository) bool {
		fields := labels.Set{
			"name": repo.Name,
		}
		return filterFields.Matches(fields)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fakeClient.WaitForWatchCompletion()

	repo := &api.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}
	repoBytes, _ := latest.Codec.Encode(repo)
	fakeClient.WatchResponse <- &etcd.Response{
		Action: "set",
		Node: &etcd.Node{
			Value: string(repoBytes),
		},
	}

	event := <-watching.ResultChan()
	if e, a := watch.Added, event.Type; e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
	if e, a := repo, event.Object; !reflect.DeepEqual(e, a) {
		t.Errorf("Expected %v, got %v", e, a)
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

func TestEtcdCreateImageFailsWithoutNamespace(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.TestIndex = true
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImage(kapi.NewContext(), &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
	})

	if err == nil {
		t.Errorf("expected error that namespace was missing from context")
	}
}

func TestEtcdListImagesInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Data["/images/alfa"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo1"}}),
					},
				},
			},
		},
		E: nil,
	}
	fakeClient.Data["/images/bravo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo2"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "bar2"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)

	buildsAlfa, err := registry.ListImages(namespaceAlfa, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(buildsAlfa.Items) != 1 || buildsAlfa.Items[0].Name != "foo1" {
		t.Errorf("Unexpected builds list: %#v", buildsAlfa)
	}

	buildsBravo, err := registry.ListImages(namespaceBravo, labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(buildsBravo.Items) != 2 || buildsBravo.Items[0].Name != "foo2" || buildsBravo.Items[1].Name != "bar2" {
		t.Errorf("Unexpected builds list: %#v", buildsBravo)
	}
}

func TestEtcdGetImageInDifferentNamespaces(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	namespaceAlfa := kapi.WithNamespace(kapi.NewContext(), "alfa")
	namespaceBravo := kapi.WithNamespace(kapi.NewContext(), "bravo")
	fakeClient.Set("/images/alfa/foo", runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	fakeClient.Set("/images/bravo/foo", runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)

	alfaFoo, err := registry.GetImage(namespaceAlfa, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if alfaFoo == nil || alfaFoo.Name != "foo" {
		t.Errorf("Unexpected deployment: %#v", alfaFoo)
	}

	bravoFoo, err := registry.GetImage(namespaceBravo, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if bravoFoo == nil || bravoFoo.Name != "foo" {
		t.Errorf("Unexpected deployment: %#v", bravoFoo)
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
