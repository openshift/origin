package controller

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

type fakeRouterPlugin struct {
	lastSyncProcessed bool
}

func (p *fakeRouterPlugin) HandleRoute(t watch.EventType, route *routeapi.Route) error {
	return nil
}
func (p *fakeRouterPlugin) HandleNode(t watch.EventType, node *kapi.Node) error {
	return nil
}
func (p *fakeRouterPlugin) HandleEndpoints(watch.EventType, *kapi.Endpoints) error {
	return nil
}
func (p *fakeRouterPlugin) HandleNamespaces(namespaces sets.String) error {
	return nil
}
func (p *fakeRouterPlugin) SetLastSyncProcessed(processed bool) error {
	p.lastSyncProcessed = processed
	return nil
}

type fakeNamespaceLister struct {
}

func (n fakeNamespaceLister) NamespaceNames() (sets.String, error) {
	return sets.NewString("foo"), nil
}

func TestRouterController_updateLastSyncProcessed(t *testing.T) {
	p := fakeRouterPlugin{}
	routesListConsumed := true
	c := RouterController{
		Plugin: &p,
		NextEndpoints: func() (watch.EventType, *kapi.Endpoints, error) {
			return watch.Modified, &kapi.Endpoints{}, nil
		},
		NextRoute: func() (watch.EventType, *routeapi.Route, error) {
			return watch.Modified, &routeapi.Route{}, nil
		},
		NextNode: func() (watch.EventType, *kapi.Node, error) {
			return watch.Modified, &kapi.Node{}, nil
		},
		EndpointsListConsumed: func() bool {
			return true
		},
		RoutesListConsumed: func() bool {
			return routesListConsumed
		},
		Namespaces:       fakeNamespaceLister{},
		NamespaceRetries: 1,
	}

	// Simulate the initial sync
	c.HandleNamespaces()
	if p.lastSyncProcessed {
		t.Fatalf("last sync not expected to have been processed")
	}
	c.HandleEndpoints()
	if p.lastSyncProcessed {
		t.Fatalf("last sync not expected to have been processed")
	}
	c.HandleRoute()
	if !p.lastSyncProcessed {
		t.Fatalf("last sync expected to have been processed")
	}

	// Simulate a relist
	routesListConsumed = false
	c.HandleRoute()
	if p.lastSyncProcessed {
		t.Fatalf("last sync not expected to have been processed")
	}
	routesListConsumed = true
	c.HandleRoute()
	if !p.lastSyncProcessed {
		t.Fatalf("last sync expected to have been processed")
	}

}
