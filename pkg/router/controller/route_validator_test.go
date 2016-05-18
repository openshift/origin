package controller

import (
	"fmt"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

type rejection struct {
	route   *routeapi.Route
	reason  string
	message string
}

type fakeRejections struct {
	rejections []rejection
}

func (r *fakeRejections) RecordRouteRejection(route *routeapi.Route, reason, message string) {
	r.rejections = append(r.rejections, rejection{route: route, reason: reason, message: message})
}

type testPlugin struct {
	route *routeapi.Route
}

func (p *testPlugin) HandleRoute(_ watch.EventType, route *routeapi.Route) error {
	p.route = route
	return nil
}
func (p *testPlugin) HandleEndpoints(watch.EventType, *kapi.Endpoints) error {
	return fmt.Errorf("not expected")
}
func (p *testPlugin) HandleNamespaces(namespaces sets.String) error {
	return fmt.Errorf("not expected")
}
func (p *testPlugin) SetLastSyncProcessed(processed bool) error {
	return fmt.Errorf("not expected")
}

// If a route with no host is added, and there is no template defined, then the
// route should be ignored.
func TestTemplateEmptyHostEmpty(t *testing.T) {
	testPlugin := &testPlugin{}
	rejections := &fakeRejections{}
	routeValidator := NewRouteValidator(testPlugin, "", false, []string{},
		rejections)
	err := routeValidator.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "route1",
			Namespace: "default",
		},
		Spec: routeapi.RouteSpec{
			Host: "",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if testPlugin.route != nil {
		t.Fatalf("route was added when it should have been ignored: %v",
			testPlugin.route)
	}

	if len(rejections.rejections) != 1 ||
		rejections.rejections[0].route.Name != "route1" ||
		rejections.rejections[0].reason != "NoHostValue" ||
		rejections.rejections[0].message != "no host value was defined for the route" {
		t.Fatalf("did not record expected rejection: %#v", rejections)
	}
}

// If a route with no host is added, and there is a template defined, then the
// route should be added with a host generated according to the template.
func TestTemplatePresentHostEmpty(t *testing.T) {
	testPlugin := &testPlugin{}
	rejections := &fakeRejections{}
	routeValidator := NewRouteValidator(testPlugin,
		"${name}-${namespace}.myapps.mycompany.com", false, []string{}, rejections)
	err := routeValidator.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "route1",
			Namespace: "default",
		},
		Spec: routeapi.RouteSpec{
			Host: "",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rejections.rejections) > 0 {
		t.Fatalf("did not expect a recorded rejection: %#v", rejections)
	}

	if testPlugin.route == nil {
		t.Fatal("route was not added when it should have been")
	}

	if testPlugin.route.Spec.Host != "route1-default.myapps.mycompany.com" {
		t.Fatalf("route was added with wrong host: %v",
			testPlugin.route)
	}
}

// If a route is added, and there is a template defined, and the override flag
// is enabled, then the route should be added with a host generated according to
// the template.
func TestTemplatePresentHostOverride(t *testing.T) {
	testPlugin := &testPlugin{}
	rejections := &fakeRejections{}
	routeValidator := NewRouteValidator(testPlugin,
		"${name}-${namespace}.myapps.mycompany.com", true, []string{}, rejections)
	err := routeValidator.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "route1",
			Namespace: "default",
		},
		Spec: routeapi.RouteSpec{
			Host: "bar-default.myapps.mycompany.com",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rejections.rejections) > 0 {
		t.Fatalf("did not expect a recorded rejection: %#v", rejections)
	}

	if testPlugin.route == nil {
		t.Fatal("route was not added when it should have been")
	}

	if testPlugin.route.Spec.Host != "route1-default.myapps.mycompany.com" {
		t.Fatalf("route was added with wrong host: %v",
			testPlugin.route)
	}
}

// If a route is added, and there is a template defined, and the override flag
// is enabled, and the override exceptions setting is specified, then the route
// should be added with the specified host if it is in a route in the exceptions
// list, or otherwise with a generated host.
func TestOverrideExceptions(t *testing.T) {
	testPlugin := &testPlugin{}
	rejections := &fakeRejections{}
	routeValidator := NewRouteValidator(testPlugin,
		"${name}-${namespace}.myapps.mycompany.com", true, []string{"foo"},
		rejections)
	err := routeValidator.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "route1",
			Namespace: "default",
		},
		Spec: routeapi.RouteSpec{
			Host: "bar-default.myapps.mycompany.com",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rejections.rejections) > 0 {
		t.Fatalf("did not expect a recorded rejection: %#v", rejections)
	}

	if testPlugin.route == nil {
		t.Fatal("route was not added when it should have been")
	}

	if testPlugin.route.Spec.Host != "route1-default.myapps.mycompany.com" {
		t.Fatalf("route was added with wrong host: %v",
			testPlugin.route)
	}

	testPlugin.route = nil
	err = routeValidator.HandleRoute(watch.Added, &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "route2",
			Namespace: "foo",
		},
		Spec: routeapi.RouteSpec{
			Host: "bar.myapps.mycompany.com",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rejections.rejections) > 0 {
		t.Fatalf("did not expect a recorded rejection: %#v", rejections)
	}

	if testPlugin.route == nil {
		t.Fatal("route was not added when it should have been")
	}

	if testPlugin.route.Spec.Host != "bar.myapps.mycompany.com" {
		t.Fatalf("route was added with wrong host: %v",
			testPlugin.route)
	}
}
