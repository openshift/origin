package route

import (
	"net/http"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/route/api"
	ractest "github.com/openshift/origin/pkg/route/controller/allocation/test"
	"github.com/openshift/origin/pkg/route/registry/test"
)

func TestListRoutesEmptyList(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	mockRegistry.Routes = &api.RouteList{
		Items: []api.Route{},
	}

	storage := REST{
		registry:  mockRegistry,
		allocator: mockAllocator,
	}

	routes, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	if len(routes.(*api.RouteList).Items) != 0 {
		t.Errorf("Unexpected non-zero routes list: %#v", routes)
	}
}

func TestListRoutesPopulatedList(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	mockRegistry.Routes = &api.RouteList{
		Items: []api.Route{
			{
				ObjectMeta: kapi.ObjectMeta{
					Name: "foo",
				},
			},
			{
				ObjectMeta: kapi.ObjectMeta{
					Name: "bar",
				},
			},
		},
	}

	storage := REST{
		registry:  mockRegistry,
		allocator: mockAllocator,
	}

	list, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), fields.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	routes := list.(*api.RouteList)

	if e, a := 2, len(routes.Items); e != a {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

func TestCreateRouteBadObject(t *testing.T) {
	storage := REST{}

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.RouteList{})
	if obj != nil {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "not a route") == -1 {
		t.Errorf("Expected 'not a route' error, got '%v'", err.Error())
	}
}

func TestCreateRouteOK(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	storage := REST{
		registry:  mockRegistry,
		allocator: mockAllocator,
	}

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.Route{
		ObjectMeta:  kapi.ObjectMeta{Name: "foo"},
		Host:        "www.frontend.com",
		ServiceName: "myrubyservice",
	})
	if obj == nil {
		t.Errorf("Expected nil obj, got %v", obj)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	route, ok := obj.(*api.Route)
	if !ok {
		t.Errorf("Expected route type, got: %#v", obj)
	}
	if route.Name != "foo" {
		t.Errorf("Unexpected route: %#v", route)
	}
	if generatedAnnotation := route.Annotations[HostGeneratedAnnotationKey]; generatedAnnotation != "false" {
		t.Errorf("Expected generated annotation to be 'false', got '%s'", generatedAnnotation)
	}
}

func TestCreateRouteGenerated(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	storage := REST{
		registry:  mockRegistry,
		allocator: mockAllocator,
	}

	obj, err := storage.Create(kapi.NewDefaultContext(), &api.Route{
		ObjectMeta:  kapi.ObjectMeta{Name: "foo"},
		ServiceName: "myrubyservice",
	})
	if obj == nil {
		t.Errorf("Expected nil obj, got %v", obj)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	route, ok := obj.(*api.Route)
	if !ok {
		t.Errorf("Expected route type, got: %#v", obj)
	}
	if route.Name != "foo" {
		t.Errorf("Unexpected route: %#v", route)
	}
	if generatedAnnotation := route.Annotations[HostGeneratedAnnotationKey]; generatedAnnotation != "true" {
		t.Errorf("Expected generated annotation to be 'true', got '%s'", generatedAnnotation)
	}
}

func TestGetRouteError(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	storage := REST{
		registry:  mockRegistry,
		allocator: mockAllocator,
	}

	route, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if route != nil {
		t.Errorf("Unexpected non-nil route: %#v", route)
	}
	expectedError := "Route foo not found"
	if err.Error() != expectedError {
		t.Errorf("Expected %#v, got %#v", expectedError, err.Error())
	}
}

func TestGetRouteOK(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	mockRegistry.Routes = &api.RouteList{
		Items: []api.Route{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
			},
		},
	}
	storage := REST{
		registry:  mockRegistry,
		allocator: mockAllocator,
	}

	route, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if route == nil {
		t.Error("Unexpected nil route")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %v", err)
	}
	if route.(*api.Route).Name != "foo" {
		t.Errorf("Unexpected route: %#v", route)
	}
}

func TestUpdateRouteBadObject(t *testing.T) {
	storage := REST{}

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.RouteList{})
	if obj != nil || created {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "not a route:") == -1 {
		t.Errorf("Expected 'not a route' error, got %v", err)
	}
}

func TestUpdateRouteMissingID(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	mockRegistry.Routes = &api.RouteList{
		Items: []api.Route{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
			},
		},
	}
	storage := REST{
		registry:  mockRegistry,
		allocator: mockAllocator,
	}

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.Route{})
	if obj != nil || created {
		t.Errorf("Expected nil, got %v", obj)
	}
	if strings.Index(err.Error(), "not found") == -1 {
		t.Errorf("Expected 'not found' error, got %v", err)
	}
}

func TestUpdateRegistryErrorSaving(t *testing.T) {
	mockRepositoryRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	storage := REST{
		registry:  mockRepositoryRegistry,
		allocator: mockAllocator,
	}

	_, _, err := storage.Update(kapi.NewDefaultContext(), &api.Route{
		ObjectMeta:  kapi.ObjectMeta{Name: "foo"},
		Host:        "www.frontend.com",
		ServiceName: "rubyservice",
	})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
}

func TestUpdateRouteOK(t *testing.T) {
	mockRepositoryRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	mockRepositoryRegistry.Routes = &api.RouteList{
		Items: []api.Route{
			{
				ObjectMeta:  kapi.ObjectMeta{Name: "bar", Namespace: kapi.NamespaceDefault},
				Host:        "www.frontend.com",
				ServiceName: "rubyservice",
			},
		},
	}

	storage := REST{
		registry:  mockRepositoryRegistry,
		allocator: mockAllocator,
	}

	obj, created, err := storage.Update(kapi.NewDefaultContext(), &api.Route{
		ObjectMeta:  kapi.ObjectMeta{Name: "bar", ResourceVersion: "foo"},
		Host:        "www.newfrontend.com",
		ServiceName: "newrubyservice",
	})

	if err != nil || created {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	route, ok := obj.(*api.Route)
	if !ok {
		t.Errorf("Expected Route, got %#v", obj)
	}
	if route == nil {
		t.Errorf("Nil route returned: %#v", route)
		t.Errorf("Expected Route, got %#v", obj)
		return
	}
	if route.Name != "bar" {
		t.Errorf("Unexpected route returned: %#v", route)
	}
	if route.Host != "www.newfrontend.com" {
		t.Errorf("Updated route not returned: %#v", route)
	}
	if route.ServiceName != "newrubyservice" {
		t.Errorf("Updated route not returned: %#v", route)
	}
}

func TestDeleteRouteError(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	storage := REST{
		registry:  mockRegistry,
		allocator: mockAllocator,
	}
	_, err := storage.Delete(kapi.NewDefaultContext(), "foo")
	if err == nil {
		t.Errorf("Unexpected nil error: %#v", err)
	}
	if err.Error() != "Route foo not found" {
		t.Errorf("Expected %#v, got %#v", "Route foo not found", err.Error())
	}
}

func TestDeleteRouteOk(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	mockRegistry.Routes = &api.RouteList{
		Items: []api.Route{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
			},
		},
	}
	storage := REST{
		registry:  mockRegistry,
		allocator: mockAllocator,
	}
	obj, err := storage.Delete(kapi.NewDefaultContext(), "foo")
	if obj == nil {
		t.Error("Unexpected nil obj")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	status, ok := obj.(*kapi.Status)
	if !ok {
		t.Errorf("Expected status type, got: %#v", obj)
	}
	if status.Status != kapi.StatusSuccess {
		t.Errorf("Expected status=success, got: %#v", status)
	}
}

func TestCreateRouteConflictingNamespace(t *testing.T) {
	storage := REST{}

	obj, err := storage.Create(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.Route{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "some-value"},
	})

	if obj != nil {
		t.Error("Expected a nil obj, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func TestUpdateRouteConflictingNamespace(t *testing.T) {
	mockRepositoryRegistry := test.NewRouteRegistry()
	mockAllocator := ractest.NewTestRouteAllocationController()
	storage := REST{
		registry:  mockRepositoryRegistry,
		allocator: mockAllocator,
	}

	obj, created, err := storage.Update(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.Route{
		ObjectMeta:  kapi.ObjectMeta{Name: "bar", Namespace: "some-value"},
		Host:        "www.newfrontend.com",
		ServiceName: "newrubyservice",
	})

	if obj != nil || created {
		t.Error("Expected a nil obj, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func checkExpectedNamespaceError(t *testing.T, err error) {
	expectedError := "Route.Namespace does not match the provided context"
	if err == nil {
		t.Errorf("Expected '" + expectedError + "', but we didn't get one")
	} else {
		e, ok := err.(kclient.APIStatus)
		if !ok {
			t.Errorf("error was not a statusError: %v", err)
		}
		if e.Status().Code != http.StatusConflict {
			t.Errorf("Unexpected failure status: %v", e.Status())
		}
		if strings.Index(err.Error(), expectedError) == -1 {
			t.Errorf("Expected '"+expectedError+"' error, got '%v'", err.Error())
		}
	}

}
