package etcd

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	etcdtesting "k8s.io/apiserver/pkg/storage/etcd/testing"
	"k8s.io/kubernetes/pkg/registry/registrytest"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	routetypes "github.com/openshift/origin/pkg/route"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	_ "github.com/openshift/origin/pkg/route/apis/route/install"
	"github.com/openshift/origin/pkg/route/registry/route"
	"github.com/openshift/origin/pkg/util/restoptions"
)

type testAllocator struct {
	Hostname string
	Err      error
	Allocate bool
	Generate bool
}

func (a *testAllocator) AllocateRouterShard(*routeapi.Route) (*routeapi.RouterShard, error) {
	a.Allocate = true
	return nil, a.Err
}
func (a *testAllocator) GenerateHostname(*routeapi.Route, *routeapi.RouterShard) string {
	a.Generate = true
	return a.Hostname
}

type testSAR struct {
	allow bool
	err   error
	sar   *authorizationapi.SubjectAccessReview
}

func (t *testSAR) CreateSubjectAccessReview(ctx apirequest.Context, subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	t.sar = subjectAccessReview
	return &authorizationapi.SubjectAccessReviewResponse{Allowed: t.allow}, t.err
}

func newStorage(t *testing.T, allocator routetypes.RouteAllocator) (*REST, *etcdtesting.EtcdTestServer) {
	etcdStorage, server := registrytest.NewEtcdStorage(t, "")
	storage, _, err := NewREST(restoptions.NewSimpleGetter(etcdStorage), allocator, &testSAR{allow: true})
	if err != nil {
		t.Fatal(err)
	}
	return storage, server
}

func validRoute() *routeapi.Route {
	return &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: routeapi.RouteSpec{
			To: routeapi.RouteTargetReference{
				Name: "test",
				Kind: "Service",
			},
		},
	}
}

func TestCreate(t *testing.T) {
	storage, server := newStorage(t, nil)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)
	test.TestCreate(
		// valid
		validRoute(),
		// invalid
		&routeapi.Route{
			ObjectMeta: metav1.ObjectMeta{Name: "_-a123-a_"},
		},
	)
}

func TestCreateWithAllocation(t *testing.T) {
	allocator := &testAllocator{Hostname: "bar"}
	storage, server := newStorage(t, allocator)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()

	obj, err := storage.Create(apirequest.NewDefaultContext(), validRoute(), false)
	if err != nil {
		t.Fatalf("unable to create object: %v", err)
	}
	result := obj.(*routeapi.Route)
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
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)

	test.TestUpdate(
		validRoute(),
		// valid update
		func(obj runtime.Object) runtime.Object {
			object := obj.(*routeapi.Route)
			if object.Annotations == nil {
				object.Annotations = map[string]string{}
			}
			object.Annotations["updated"] = "true"
			return object
		},
		// invalid update
		func(obj runtime.Object) runtime.Object {
			object := obj.(*routeapi.Route)
			object.Spec.Path = "invalid/path"
			return object
		},
	)
}

func TestList(t *testing.T) {
	storage, server := newStorage(t, nil)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)
	test.TestList(
		validRoute(),
	)
}

func TestGet(t *testing.T) {
	storage, server := newStorage(t, &testAllocator{})
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)
	test.TestGet(
		validRoute(),
	)
}

func TestDelete(t *testing.T) {
	storage, server := newStorage(t, nil)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)
	test.TestDelete(
		validRoute(),
	)
}

func TestWatch(t *testing.T) {
	storage, server := newStorage(t, nil)
	defer server.Terminate(t)
	defer storage.Store.DestroyFunc()
	test := registrytest.New(t, storage.Store)

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
