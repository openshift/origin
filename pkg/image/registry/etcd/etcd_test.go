package etcd

import (
	"fmt"
	"reflect"
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
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

func NewTestEtcd(client tools.EtcdClient) *Etcd {
	return New(tools.EtcdHelper{client, latest.Codec, latest.ResourceVersioner})
}

func TestEtcdListImagesEmpty(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/images"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	images, err := registry.ListImages(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(images.Items) != 0 {
		t.Errorf("Unexpected images list: %#v", images)
	}
}

func TestEtcdListImagesError(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/images"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	images, err := registry.ListImages(labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if images != nil {
		t.Errorf("Unexpected non-nil images: %#v", images)
	}
}

func TestEtcdListImagesEverything(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/images"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{JSONBase: kubeapi.JSONBase{ID: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	images, err := registry.ListImages(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(images.Items) != 2 || images.Items[0].ID != "foo" || images.Items[1].ID != "bar" {
		t.Errorf("Unexpected images list: %#v", images)
	}
}

func TestEtcdListImagesFiltered(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/images"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{
							JSONBase: kubeapi.JSONBase{ID: "foo"},
							Labels:   map[string]string{"env": "prod"},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.Image{
							JSONBase: kubeapi.JSONBase{ID: "bar"},
							Labels:   map[string]string{"env": "dev"},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	images, err := registry.ListImages(labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(images.Items) != 1 || images.Items[0].ID != "bar" {
		t.Errorf("Unexpected images list: %#v", images)
	}
}

func TestEtcdGetImage(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set("/images/foo", runtime.EncodeOrDie(latest.Codec, &api.Image{JSONBase: kubeapi.JSONBase{ID: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	image, err := registry.GetImage("foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if image.ID != "foo" {
		t.Errorf("Unexpected image: %#v", image)
	}
}

func TestEtcdGetImageNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/images/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	image, err := registry.GetImage("foo")
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
	fakeClient.Data["/images/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImage(&api.Image{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
		DockerImageReference: "openshift/ruby-19-centos",
		Metadata: docker.Image{
			ID: "abc123",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get("/images/foo", false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var image api.Image
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &image)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if image.ID != "foo" {
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
	fakeClient.Data["/images/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.Image{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImage(&api.Image{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
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
	err := registry.UpdateImage(&api.Image{})
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteImageNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteImage("foo")
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
	err := registry.DeleteImage("foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteImageOK(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	key := "/images/foo"
	err := registry.DeleteImage("foo")
	if err != nil {
		t.Errorf("Unexpected error: %#v", err)
	}
	if len(fakeClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeClient.DeletedKeys)
	} else if fakeClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeClient.DeletedKeys[0], key)
	}
}

func TestEtcdListImagesRepositoriesEmpty(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/imageRepositories"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	repos, err := registry.ListImageRepositories(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(repos.Items) != 0 {
		t.Errorf("Unexpected image repositories list: %#v", repos)
	}
}

func TestEtcdListImageRepositoriesError(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/imageRepositories"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: fmt.Errorf("some error"),
	}
	registry := NewTestEtcd(fakeClient)
	repos, err := registry.ListImageRepositories(labels.Everything())
	if err == nil {
		t.Error("unexpected nil error")
	}

	if repos != nil {
		t.Errorf("Unexpected non-nil repos: %#v", repos)
	}
}

func TestEtcdListImageRepositoriesEverything(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/imageRepositories"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{JSONBase: kubeapi.JSONBase{ID: "bar"}}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	repos, err := registry.ListImageRepositories(labels.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(repos.Items) != 2 || repos.Items[0].ID != "foo" || repos.Items[1].ID != "bar" {
		t.Errorf("Unexpected images list: %#v", repos)
	}
}

func TestEtcdListImageRepositoriesFiltered(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	key := "/imageRepositories"
	fakeClient.Data[key] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{
							JSONBase: kubeapi.JSONBase{ID: "foo"},
							Labels:   map[string]string{"env": "prod"},
						}),
					},
					{
						Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{
							JSONBase: kubeapi.JSONBase{ID: "bar"},
							Labels:   map[string]string{"env": "dev"},
						}),
					},
				},
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	repos, err := registry.ListImageRepositories(labels.SelectorFromSet(labels.Set{"env": "dev"}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(repos.Items) != 1 || repos.Items[0].ID != "bar" {
		t.Errorf("Unexpected repos list: %#v", repos)
	}
}

func TestEtcdGetImageRepository(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Set("/imageRepositories/foo", runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{JSONBase: kubeapi.JSONBase{ID: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	repo, err := registry.GetImageRepository("foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if repo.ID != "foo" {
		t.Errorf("Unexpected repo: %#v", repo)
	}
}

func TestEtcdGetImageRepositoryNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Data["/imageRepositories/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	repo, err := registry.GetImageRepository("foo")
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
	fakeClient.Data["/imageRepositories/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImageRepository(&api.ImageRepository{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
		},
		Labels:                map[string]string{"a": "b"},
		DockerImageRepository: "c/d",
		Tags: map[string]string{"t1": "v1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := fakeClient.Get("/imageRepositories/foo", false, false)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	var repo api.ImageRepository
	err = latest.Codec.DecodeInto([]byte(resp.Node.Value), &repo)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if repo.ID != "foo" {
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
	fakeClient.Data["/imageRepositories/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{JSONBase: kubeapi.JSONBase{ID: "foo"}}),
			},
		},
		E: nil,
	}
	registry := NewTestEtcd(fakeClient)
	err := registry.CreateImageRepository(&api.ImageRepository{
		JSONBase: kubeapi.JSONBase{
			ID: "foo",
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

	resp, _ := fakeClient.Set("/imageRepositories/foo", runtime.EncodeOrDie(latest.Codec, &api.ImageRepository{JSONBase: kubeapi.JSONBase{ID: "foo"}}), 0)
	registry := NewTestEtcd(fakeClient)
	err := registry.UpdateImageRepository(&api.ImageRepository{
		JSONBase:              kubeapi.JSONBase{ID: "foo", ResourceVersion: resp.Node.ModifiedIndex},
		DockerImageRepository: "some/repo",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	repo, err := registry.GetImageRepository("foo")
	if repo.DockerImageRepository != "some/repo" {
		t.Errorf("Unexpected repo: %#v", repo)
	}
}

func TestEtcdDeleteImageRepositoryNotFound(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	fakeClient.Err = tools.EtcdErrorNotFound
	registry := NewTestEtcd(fakeClient)
	err := registry.DeleteImageRepository("foo")
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
	err := registry.DeleteImageRepository("foo")
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestEtcdDeleteImageRepositoryOK(t *testing.T) {
	fakeClient := tools.NewFakeEtcdClient(t)
	registry := NewTestEtcd(fakeClient)
	key := "/imageRepositories/foo"
	err := registry.DeleteImageRepository("foo")
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
	filterFields := labels.SelectorFromSet(labels.Set{"ID": "foo"})

	watching, err := registry.WatchImageRepositories(1, func(repo *api.ImageRepository) bool {
		fields := labels.Set{
			"ID": repo.ID,
		}
		return filterFields.Matches(fields)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fakeClient.WaitForWatchCompletion()

	repo := &api.ImageRepository{JSONBase: kubeapi.JSONBase{ID: "foo"}}
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
