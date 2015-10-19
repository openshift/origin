package etcd

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/go-etcd/etcd"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	"k8s.io/kubernetes/pkg/tools"
	"k8s.io/kubernetes/pkg/tools/etcdtest"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
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

func newHelper(t *testing.T) (*tools.FakeEtcdClient, storage.Interface) {
	fakeEtcdClient := tools.NewFakeEtcdClient(t)
	fakeEtcdClient.TestIndex = true
	helper := etcdstorage.NewEtcdStorage(fakeEtcdClient, latest.Codec, etcdtest.PathPrefix())
	return fakeEtcdClient, helper
}

func newStorage(t *testing.T) (*REST, *tools.FakeEtcdClient) {
	etcdStorage, fakeClient := registrytest.NewEtcdStorage(t, "")
	return NewREST(etcdStorage), fakeClient
}

func TestStorage(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)
	image.NewRegistry(storage)
}

func validNewImage() *api.Image {
	return &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
		DockerImageReference: "openshift/origin",
	}
}

func TestCreate(t *testing.T) {
	storage, fakeClient := newStorage(t)
	test := registrytest.New(t, fakeClient, storage.store).ClusterScope()
	image := validNewImage()
	image.ObjectMeta = kapi.ObjectMeta{GenerateName: "foo"}
	test.TestCreate(
		// valid
		image,
		// invalid
		&api.Image{},
	)
}

func TestCreateRegistryError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("test error")
	storage := NewREST(helper)

	image := validNewImage()
	_, err := storage.Create(kapi.NewDefaultContext(), image)
	if err != fakeEtcdClient.Err {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateAlreadyExists(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.TestIndex = true

	storage := NewREST(helper)

	existingImage := &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name:            "foo",
			ResourceVersion: "1",
		},
		DockerImageReference: "foo/bar:abcd1234",
	}

	fakeEtcdClient.Data["/images/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value:         runtime.EncodeOrDie(latest.Codec, existingImage),
				CreatedIndex:  1,
				ModifiedIndex: 1,
			},
		},
	}
	_, err := storage.Create(kapi.NewDefaultContext(), &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
		DockerImageReference: "foo/bar:abcd1234",
	})
	if err == nil {
		t.Fatalf("Unexpected non error")
	}
	if !errors.IsAlreadyExists(err) {
		t.Errorf("Expected already exists error, got %s", err)
	}
}

func TestListError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("test error")
	storage := NewREST(helper)
	images, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != fakeEtcdClient.Err {
		t.Fatalf("Expected %#v, Got %#v", fakeEtcdClient.Err, err)
	}
	if images != nil {
		t.Errorf("Unexpected non-nil image list: %#v", images)
	}
}

func TestListEmptyList(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.ChangeIndex = 1
	fakeEtcdClient.Data["/images"] = tools.EtcdResponseWithError{
		R: &etcd.Response{},
		E: fakeEtcdClient.NewError(tools.EtcdErrorCodeNotFound),
	}
	storage := NewREST(helper)
	images, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	if len(images.(*api.ImageList).Items) != 0 {
		t.Errorf("Unexpected non-zero images list: %#v", images)
	}
	if images.(*api.ImageList).ResourceVersion != "1" {
		t.Errorf("Unexpected resource version: %#v", images)
	}
}

func TestListPopulatedList(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.ChangeIndex = 1
	fakeEtcdClient.Data["/images"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Nodes: []*etcd.Node{
					{Value: runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})},
					{Value: runtime.EncodeOrDie(latest.Codec, &api.Image{ObjectMeta: kapi.ObjectMeta{Name: "bar"}})},
				},
			},
		},
	}

	storage := NewREST(helper)

	list, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	images := list.(*api.ImageList)

	if e, a := 2, len(images.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestListFiltered(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.ChangeIndex = 1
	fakeEtcdClient.Data["/images"] = tools.EtcdResponseWithError{
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
	storage := NewREST(helper)
	list, err := storage.List(kapi.NewDefaultContext(), labels.SelectorFromSet(labels.Set{"env": "dev"}), fields.Everything())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	images := list.(*api.ImageList)
	if len(images.Items) != 1 || images.Items[0].Name != "bar" {
		t.Errorf("Unexpected images list: %#v", images)
	}
}

func TestCreateMissingID(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.Image{})
	if obj != nil {
		t.Errorf("Expected nil obj, got %v", obj)
	}
	if !errors.IsInvalid(err) {
		t.Errorf("Expected 'invalid' error, got %v", err)
	}
}

func TestCreateOK(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.Image{
		ObjectMeta:           kapi.ObjectMeta{Name: "foo"},
		DockerImageReference: "openshift/ruby-19-centos",
	})
	if obj == nil {
		t.Errorf("Expected nil obj, got %v", obj)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	image, ok := obj.(*api.Image)
	if !ok {
		t.Errorf("Expected image type, got: %#v", obj)
	}
	if image.Name != "foo" {
		t.Errorf("Unexpected image: %#v", image)
	}
}

func TestGetError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("bad")
	storage := NewREST(helper)

	image, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if image != nil {
		t.Errorf("Unexpected non-nil image: %#v", image)
	}
	if err != fakeEtcdClient.Err {
		t.Errorf("Expected %v, got %v", fakeEtcdClient.Err, err)
	}
}

func TestGetNotFound(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage := NewREST(helper)
	fakeEtcdClient.Data["/images/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: nil,
		},
		E: tools.EtcdErrorNotFound,
	}

	image, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Errorf("Unexpected non-error.")
	}
	if image != nil {
		t.Errorf("Unexpected image: %#v", image)
	}
}

func TestGetOK(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	expectedImage := &api.Image{
		ObjectMeta:           kapi.ObjectMeta{Name: "foo"},
		DockerImageReference: "openshift/ruby-19-centos",
	}
	fakeEtcdClient.Data["/images/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, expectedImage),
			},
		},
	}
	storage := NewREST(helper)

	image, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if image == nil {
		t.Fatal("Unexpected nil image")
	}
	if err != nil {
		t.Fatal("Unexpected non-nil error", err)
	}
	if image.(*api.Image).Name != "foo" {
		t.Errorf("Unexpected image: %#v", image)
	}
}

func TestDelete(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Data["/images/foo"] = tools.EtcdResponseWithError{
		R: &etcd.Response{
			Node: &etcd.Node{
				Value: runtime.EncodeOrDie(latest.Codec, &api.Image{}),
			},
		},
	}
	storage := NewREST(helper)

	obj, err := storage.Delete(kapi.NewDefaultContext(), "foo", nil)

	if obj == nil {
		t.Error("Unexpected nil obj")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	status, ok := obj.(*kapi.Status)
	if !ok {
		t.Fatalf("Expected status type, got: %#v", obj)
	}
	if status.Status != kapi.StatusSuccess {
		t.Errorf("Expected status=success, got: %#v", status)
	}
	if len(fakeEtcdClient.DeletedKeys) != 1 {
		t.Errorf("Expected 1 delete, found %#v", fakeEtcdClient.DeletedKeys)
	} else if key := "/images/foo"; fakeEtcdClient.DeletedKeys[0] != key {
		t.Errorf("Unexpected key: %s, expected %s", fakeEtcdClient.DeletedKeys[0], key)
	}
}

func TestDeleteNotFound(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = tools.EtcdErrorNotFound
	storage := NewREST(helper)
	_, err := storage.Delete(kapi.NewDefaultContext(), "foo", nil)
	if err == nil {
		t.Error("Unexpected non-error")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("Expected 'not found' error, got %#v", err)
	}
}

func TestDeleteImageError(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	fakeEtcdClient.Err = fmt.Errorf("Some error")
	storage := NewREST(helper)
	_, err := storage.Delete(kapi.NewDefaultContext(), "foo", nil)
	if err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestWatchErrorWithFieldSet(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)

	_, err := storage.Watch(kapi.NewDefaultContext(), labels.Everything(), fields.SelectorFromSet(fields.Set{"foo": "bar"}), "1")
	if err == nil {
		t.Fatal("unexpected nil error")
	}
	if err.Error() != "field selectors are not supported on images" {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestWatchOK(t *testing.T) {
	fakeEtcdClient, helper := newHelper(t)
	storage := NewREST(helper)

	var tests = []struct {
		label    labels.Selector
		images   []*api.Image
		expected []bool
	}{
		{
			labels.Everything(),
			[]*api.Image{
				{ObjectMeta: kapi.ObjectMeta{Name: "a"}, DockerImageMetadata: api.DockerImage{}},
				{ObjectMeta: kapi.ObjectMeta{Name: "b"}, DockerImageMetadata: api.DockerImage{}},
				{ObjectMeta: kapi.ObjectMeta{Name: "c"}, DockerImageMetadata: api.DockerImage{}},
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
				{ObjectMeta: kapi.ObjectMeta{Name: "a", Labels: map[string]string{"color": "blue"}}, DockerImageMetadata: api.DockerImage{}},
				{ObjectMeta: kapi.ObjectMeta{Name: "b", Labels: map[string]string{"color": "green"}}, DockerImageMetadata: api.DockerImage{}},
				{ObjectMeta: kapi.ObjectMeta{Name: "c", Labels: map[string]string{"color": "blue"}}, DockerImageMetadata: api.DockerImage{}},
			},
			[]bool{
				true,
				false,
				true,
			},
		},
	}
	for _, tt := range tests {
		watching, err := storage.Watch(kapi.NewDefaultContext(), tt.label, fields.Everything(), "1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		fakeEtcdClient.WaitForWatchCompletion()

		for testIndex, image := range tt.images {
			imageBytes, _ := latest.Codec.Encode(image)
			fakeEtcdClient.WatchResponse <- &etcd.Response{
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
				image.DockerImageMetadataVersion = "1.0"
				if e, a := image, event.Object; !reflect.DeepEqual(e, a) {
					t.Errorf("Objects did not match: %s", util.ObjectDiff(e, a))
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

		fakeEtcdClient.WatchInjectError <- nil
		if _, ok := <-watching.ResultChan(); ok {
			t.Errorf("watching channel should be closed")
		}
		watching.Stop()
	}
}

type fakeStrategy struct {
	rest.RESTCreateStrategy
}

func (fakeStrategy) PrepareForCreate(obj runtime.Object) {
	img := obj.(*api.Image)
	img.Annotations = make(map[string]string, 1)
	img.Annotations["test"] = "PrepareForCreate"
}

func TestStrategyPrepareMethods(t *testing.T) {
	_, helper := newHelper(t)
	storage := NewREST(helper)
	img := validNewImage()
	strategy := fakeStrategy{image.Strategy}

	storage.store.CreateStrategy = strategy

	obj, err := storage.Create(kapi.NewDefaultContext(), img)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	newImage := obj.(*api.Image)
	if newImage.Annotations["test"] != "PrepareForCreate" {
		t.Errorf("Expected PrepareForCreate annotation")
	}
}
