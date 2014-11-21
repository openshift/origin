package route

import (
	"net/http"
	"strings"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/route/registry/test"
)

func TestListRoutesEmptyList(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	mockRegistry.Routes = &api.RouteList{
		Items: []api.Route{},
	}

	storage := REST{
		registry: mockRegistry,
	}

	routes, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), labels.Everything())
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	if len(routes.(*api.RouteList).Items) != 0 {
		t.Errorf("Unexpected non-zero routes list: %#v", routes)
	}
}

func TestListRoutesPopulatedList(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
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
		registry: mockRegistry,
	}

	list, err := storage.List(kapi.NewDefaultContext(), labels.Everything(), labels.Everything())
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

	channel, err := storage.Create(kapi.NewDefaultContext(), &api.RouteList{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not a route") == -1 {
		t.Errorf("Expected 'not a route' error, got '%v'", err.Error())
	}
}

func TestCreateRouteOK(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	storage := REST{registry: mockRegistry}

	channel, err := storage.Create(kapi.NewDefaultContext(), &api.Route{
		ObjectMeta:  kapi.ObjectMeta{Name: "foo"},
		Host:        "www.frontend.com",
		ServiceName: "myrubyservice",
	})
	if channel == nil {
		t.Errorf("Expected nil channel, got %v", channel)
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	select {
	case result := <-channel:
		route, ok := result.Object.(*api.Route)
		if !ok {
			t.Errorf("Expected route type, got: %#v", result)
		}
		if route.Name != "foo" {
			t.Errorf("Unexpected route: %#v", route)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	}
}

func TestGetRouteError(t *testing.T) {
	mockRegistry := test.NewRouteRegistry()
	storage := REST{registry: mockRegistry}

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
	mockRegistry.Routes = &api.RouteList{
		Items: []api.Route{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
			},
		},
	}
	storage := REST{registry: mockRegistry}

	route, err := storage.Get(kapi.NewDefaultContext(), "foo")
	if route == nil {
		t.Error("Unexpected nil route")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error", err)
	}
	if route.(*api.Route).Name != "foo" {
		t.Errorf("Unexpected route: %#v", route)
	}
}

func TestUpdateRouteBadObject(t *testing.T) {
	storage := REST{}

	channel, err := storage.Update(kapi.NewDefaultContext(), &api.RouteList{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "not a route:") == -1 {
		t.Errorf("Expected 'not a route' error, got %v", err)
	}
}

func TestUpdateRouteMissingID(t *testing.T) {
	storage := REST{}

	channel, err := storage.Update(kapi.NewDefaultContext(), &api.Route{})
	if channel != nil {
		t.Errorf("Expected nil, got %v", channel)
	}
	if strings.Index(err.Error(), "name is unspecified:") == -1 {
		t.Errorf("Expected 'name is unspecified' error, got %v", err)
	}
}

func TestUpdateRegistryErrorSaving(t *testing.T) {
	mockRepositoryRegistry := test.NewRouteRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Update(kapi.NewDefaultContext(), &api.Route{
		ObjectMeta:  kapi.ObjectMeta{Name: "foo"},
		Host:        "www.frontend.com",
		ServiceName: "rubyservice",
	})
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	result := <-channel
	status, ok := result.Object.(*kapi.Status)
	if !ok {
		t.Errorf("Expected status, got %#v", result)
	}
	if status.Status != kapi.StatusFailure || status.Message != "Route foo not found" {
		t.Errorf("Expected status=failure, message=Route foo not found, got %#v", status)
	}
}

func TestUpdateRouteOK(t *testing.T) {
	mockRepositoryRegistry := test.NewRouteRegistry()
	mockRepositoryRegistry.Routes = &api.RouteList{
		Items: []api.Route{
			{
				ObjectMeta:  kapi.ObjectMeta{Name: "bar"},
				Host:        "www.frontend.com",
				ServiceName: "rubyservice",
			},
		},
	}

	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Update(kapi.NewDefaultContext(), &api.Route{
		ObjectMeta:  kapi.ObjectMeta{Name: "bar"},
		Host:        "www.newfrontend.com",
		ServiceName: "newrubyservice",
	})

	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}
	result := <-channel
	route, ok := result.Object.(*api.Route)
	if !ok {
		t.Errorf("Expected Route, got %#v", result)
	}
	if route == nil {
		t.Errorf("Nil route returned: %#v", route)
		t.Errorf("Expected Route, got %#v", result)
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
	storage := REST{registry: mockRegistry}
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
	mockRegistry.Routes = &api.RouteList{
		Items: []api.Route{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "foo"},
			},
		},
	}
	storage := REST{registry: mockRegistry}
	channel, err := storage.Delete(kapi.NewDefaultContext(), "foo")
	if channel == nil {
		t.Error("Unexpected nil channel")
	}
	if err != nil {
		t.Errorf("Unexpected non-nil error: %#v", err)
	}

	select {
	case result := <-channel:
		status, ok := result.Object.(*kapi.Status)
		if !ok {
			t.Errorf("Expected status type, got: %#v", result)
		}
		if status.Status != kapi.StatusSuccess {
			t.Errorf("Expected status=success, got: %#v", status)
		}
	case <-time.After(50 * time.Millisecond):
		t.Errorf("Timed out waiting for result")
	}
}

func TestCreateRouteConflictingNamespace(t *testing.T) {
	storage := REST{}

	channel, err := storage.Create(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.Route{
		ObjectMeta: kapi.ObjectMeta{Name: "foo", Namespace: "some-value"},
	})

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
	}

	checkExpectedNamespaceError(t, err)
}

func TestUpdateRouteConflictingNamespace(t *testing.T) {
	mockRepositoryRegistry := test.NewRouteRegistry()
	storage := REST{registry: mockRepositoryRegistry}

	channel, err := storage.Update(kapi.WithNamespace(kapi.NewContext(), "legal-name"), &api.Route{
		ObjectMeta:  kapi.ObjectMeta{Name: "bar", Namespace: "some-value"},
		Host:        "www.newfrontend.com",
		ServiceName: "newrubyservice",
	})

	if channel != nil {
		t.Error("Expected a nil channel, but we got a value")
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
