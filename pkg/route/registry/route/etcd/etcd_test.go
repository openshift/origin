package etcd

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	"k8s.io/kubernetes/pkg/runtime"
	etcdtesting "k8s.io/kubernetes/pkg/storage/etcd/testing"

	"github.com/openshift/origin/pkg/route/api"
	_ "github.com/openshift/origin/pkg/route/api/install"
	"github.com/openshift/origin/pkg/route/registry/route"
)

type testAllocator struct {
	Hostname string
	Err      error
	Allocate bool
	Generate bool
}

func (a *testAllocator) AllocateRouterShard(*api.Route) (*api.RouterShard, error) {
	a.Allocate = true
	return nil, a.Err
}
func (a *testAllocator) GenerateHostname(*api.Route, *api.RouterShard) string {
	a.Generate = true
	return a.Hostname
}

func newStorage(t *testing.T, allocator *testAllocator) (*REST, *etcdtesting.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	storage, _ := NewREST(etcdStorage, allocator)
	return storage, server
}

func validRoute() *api.Route {
	return &api.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name: "foo",
		},
		Spec: api.RouteSpec{
			To: kapi.ObjectReference{
				Name: "test",
				Kind: "Service",
			},
		},
	}
}

func TestCreate(t *testing.T) {
	storage, server := newStorage(t, &testAllocator{})
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Etcd)
	test.TestCreate(
		// valid
		validRoute(),
		// invalid
		&api.Route{
			ObjectMeta: kapi.ObjectMeta{Name: "_-a123-a_"},
		},
	)
}

func TestCreateWithAllocation(t *testing.T) {
	allocator := &testAllocator{Hostname: "bar"}
	storage, server := newStorage(t, allocator)
	defer server.Terminate(t)

	obj, err := storage.Create(kapi.NewDefaultContext(), validRoute())
	if err != nil {
		t.Fatalf("unable to create object: %v", err)
	}
	result := obj.(*api.Route)
	if result.Spec.Host != "bar" {
		t.Fatalf("unexpected route: %#v", result)
	}
	if v, ok := result.Annotations[route.HostGeneratedAnnotationKey]; !ok || v != "true" {
		t.Fatalf("unexpected route: %#v", result)
	}
	if !allocator.Allocate || !allocator.Generate {
		t.Fatalf("unexpected allocator: %#v", allocator)
	}
}

func TestUpdate(t *testing.T) {
	storage, server := newStorage(t, nil)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Etcd)

	test.TestUpdate(
		validRoute(),
		// valid update
		func(obj runtime.Object) runtime.Object {
			object := obj.(*api.Route)
			if object.Annotations == nil {
				object.Annotations = map[string]string{}
			}
			object.Annotations["updated"] = "true"
			return object
		},
		// invalid update
		func(obj runtime.Object) runtime.Object {
			object := obj.(*api.Route)
			object.Spec.Path = "invalid/path"
			return object
		},
	)
}
func TestList(t *testing.T) {
	storage, server := newStorage(t, nil)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Etcd)
	test.TestList(
		validRoute(),
	)
}

func TestGet(t *testing.T) {
	storage, server := newStorage(t, &testAllocator{})
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Etcd)
	test.TestGet(
		validRoute(),
	)
}

func TestDelete(t *testing.T) {
	storage, server := newStorage(t, nil)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Etcd)
	test.TestDelete(
		validRoute(),
	)
}

func TestWatch(t *testing.T) {
	storage, server := newStorage(t, nil)
	defer server.Terminate(t)
	test := registrytest.New(t, storage.Etcd)

	valid := validRoute()
	valid.Name = "foo"
	valid.Labels = map[string]string{"foo": "bar"}

	test.TestWatch(
		valid,
		// matching labels
		[]labels.Set{{"foo": "bar"}},
		// not matching labels
		[]labels.Set{{"foo": "baz"}},
		// matching fields
		[]fields.Set{
			{"metadata.name": "foo"},
		},
		// not matching fields
		[]fields.Set{
			{"metadata.name": "bar"},
		},
	)
}
