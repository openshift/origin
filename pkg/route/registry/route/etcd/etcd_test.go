package etcd

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/registrytest"
	"k8s.io/kubernetes/pkg/runtime"
	etcdtesting "k8s.io/kubernetes/pkg/storage/etcd/testing"

	routetypes "github.com/openshift/origin/pkg/route"
	"github.com/openshift/origin/pkg/route/api"
	_ "github.com/openshift/origin/pkg/route/api/install"
	"github.com/openshift/origin/pkg/route/registry/route"
	"github.com/openshift/origin/pkg/util/restoptions"
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

func newStorage(t *testing.T, allocator routetypes.RouteAllocator) (*REST, *etcdtesting.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	storage, _, err := NewREST(restoptions.NewSimpleGetter(etcdStorage), allocator)
	if err != nil {
		t.Fatal(err)
	}
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
	storage, server := newStorage(t, nil)
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

func TestUpdateWithAllocation(t *testing.T) {
	allocator := &testAllocator{Hostname: "bar"}
	storage, server := newStorage(t, allocator)
	defer server.Terminate(t)

	// create a route with a populated host
	originalRoute := validRoute()
	originalRoute.Spec.Host = "foo"
	created, err := storage.Create(kapi.NewDefaultContext(), originalRoute)
	if err != nil {
		t.Fatalf("error creating valid route to test allocations: %v", err)
	}

	createdRoute := created.(*api.Route)
	if createdRoute.Spec.Host != "foo" {
		t.Fatalf("unexpected host on createdRoute: %#v", createdRoute)
	}
	if _, ok := createdRoute.Annotations[route.HostGeneratedAnnotationKey]; ok {
		t.Fatalf("created route should not have the generated host annotation")
	}

	// update the route to set the host to empty
	createdRoute.Spec.Host = ""
	updated, _, err := storage.Update(kapi.NewDefaultContext(), createdRoute)
	if err != nil {
		t.Fatalf("error updating route to test allocations: %v", err)
	}

	// route should now have the allocated host of bar and the generated host annotation
	updatedRoute := updated.(*api.Route)
	if updatedRoute == nil {
		t.Fatalf("expected updatedRoute to not be nil")
	}
	if updatedRoute.Spec.Host != "bar" {
		t.Fatalf("unexpected route: %#v", updatedRoute)
	}
	if v, ok := updatedRoute.Annotations[route.HostGeneratedAnnotationKey]; !ok || v != "true" {
		t.Fatalf("unexpected route: %#v", updatedRoute)
	}
	if !allocator.Allocate || !allocator.Generate {
		t.Fatalf("unexpected allocator: %#v", allocator)
	}
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
