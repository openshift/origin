package haproxy

import (
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routev1 "github.com/openshift/api/route/v1"
	templaterouter "github.com/openshift/origin/pkg/router/template"
)

type fakeConfigManager struct {
	blueprints map[string]*routev1.Route
}

func newFakeConfigManager() *fakeConfigManager {
	return &fakeConfigManager{
		blueprints: make(map[string]*routev1.Route),
	}
}

func (cm *fakeConfigManager) Initialize(router templaterouter.RouterInterface, certPath string) {
}

func (cm *fakeConfigManager) AddBlueprint(route *routev1.Route) {
	cm.blueprints[routeKey(route)] = route
}

func (cm *fakeConfigManager) RemoveBlueprint(route *routev1.Route) {
	delete(cm.blueprints, routeKey(route))
}

func (cm *fakeConfigManager) FindBlueprint(id string) (*routev1.Route, bool) {
	route, ok := cm.blueprints[id]
	return route, ok
}

func (cm *fakeConfigManager) Register(id string, route *routev1.Route) {
}

func (cm *fakeConfigManager) AddRoute(id, routingKey string, route *routev1.Route) error {
	return nil
}

func (cm *fakeConfigManager) RemoveRoute(id string, route *routev1.Route) error {
	return nil
}

func (cm *fakeConfigManager) ReplaceRouteEndpoints(id string, oldEndpoints, newEndpoints []templaterouter.Endpoint, weight int32) error {
	return nil
}

func (cm *fakeConfigManager) RemoveRouteEndpoints(id string, endpoints []templaterouter.Endpoint) error {
	return nil
}

func (cm *fakeConfigManager) Notify(event templaterouter.RouterEventType) {
}

func (cm *fakeConfigManager) ServerTemplateName(id string) string {
	return "fakeConfigManager"
}

func (cm *fakeConfigManager) ServerTemplateSize(id string) string {
	return "1"
}

func (cm *fakeConfigManager) GenerateDynamicServerNames(id string) []string {
	return []string{}
}

func routeKey(route *routev1.Route) string {
	return fmt.Sprintf("%s:%s", route.Name, route.Namespace)
}

// TestHandleRoute test route watch events
func TestHandleRoute(t *testing.T) {
	original := metav1.Time{Time: time.Now()}

	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: original,
			Namespace:         "bp",
			Name:              "chevron",
		},
		Spec: routev1.RouteSpec{
			Host: "www.blueprints.org",
			To: routev1.RouteTargetReference{
				Name:   "TestService",
				Weight: new(int32),
			},
		},
	}

	cm := newFakeConfigManager()
	plugin := NewBlueprintPlugin(cm)
	plugin.HandleRoute(watch.Added, route)

	id := routeKey(route)
	if _, ok := cm.FindBlueprint(id); !ok {
		t.Errorf("TestHandleRoute was unable to find a blueprint %s after HandleRoute was called", id)
	}

	// update a blueprint with a newer time and host
	v2route := route.DeepCopy()
	v2route.CreationTimestamp = metav1.Time{Time: original.Add(time.Hour)}
	v2route.Spec.Host = "updated.blueprint.org"
	if err := plugin.HandleRoute(watch.Added, v2route); err != nil {
		t.Errorf("TestHandleRoute unexpected error after blueprint update: %v", err)
	}

	blueprints := []*routev1.Route{v2route, route}
	for _, r := range blueprints {
		// delete the blueprint and check that it doesn't exist.
		if err := plugin.HandleRoute(watch.Deleted, v2route); err != nil {
			t.Errorf("TestHandleRoute unexpected error after blueprint delete: %v", err)
		}

		routeId := routeKey(r)
		if _, ok := cm.FindBlueprint(routeId); ok {
			t.Errorf("TestHandleRoute found a blueprint %s after it was deleted", routeId)
		}
	}
}

func TestHandleNode(t *testing.T) {
	node := &kapi.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"design": "blueprint"},
		},
	}

	cm := newFakeConfigManager()
	plugin := NewBlueprintPlugin(cm)

	if err := plugin.HandleNode(watch.Added, node); err != nil {
		t.Errorf("TestHandleNode unexpected error after node add: %v", err)
	}

	if err := plugin.HandleNode(watch.Modified, node); err != nil {
		t.Errorf("TestHandleNode unexpected error after node modify: %v", err)
	}

	if err := plugin.HandleNode(watch.Deleted, node); err != nil {
		t.Errorf("TestHandleNode unexpected error after node delete: %v", err)
	}
}

func TestHandleEndpoints(t *testing.T) {
	endpoints := &kapi.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "bpe",
			Name:      "shell",
		},
		Subsets: []kapi.EndpointSubset{{
			Addresses: []kapi.EndpointAddress{{IP: "1.1.1.1"}},
			Ports:     []kapi.EndpointPort{{Port: 9876}},
		}},
	}

	v2Endpoints := &kapi.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "bpe",
			Name:      "shell",
		},
		Subsets: []kapi.EndpointSubset{{
			Addresses: []kapi.EndpointAddress{{IP: "1.1.1.1"}, {IP: "2.2.2.2"}},
			Ports:     []kapi.EndpointPort{{Port: 9876}, {Port: 8888}},
		}},
	}

	cm := newFakeConfigManager()
	plugin := NewBlueprintPlugin(cm)

	if err := plugin.HandleEndpoints(watch.Added, endpoints); err != nil {
		t.Errorf("TestHandleEndpoints unexpected error after endpoints add: %v", err)
	}

	if err := plugin.HandleEndpoints(watch.Modified, v2Endpoints); err != nil {
		t.Errorf("TestHandleEndpoints unexpected error after endpoints modify: %v", err)
	}

	if err := plugin.HandleEndpoints(watch.Deleted, v2Endpoints); err != nil {
		t.Errorf("TestHandleEndpoints unexpected error after endpoints delete: %v", err)
	}
}

func TestHandleNamespaces(t *testing.T) {
	cm := newFakeConfigManager()
	plugin := NewBlueprintPlugin(cm)

	if err := plugin.HandleNamespaces(sets.String{}); err != nil {
		t.Errorf("TestHandleNamespaces unexpected error after empty set: %v", err)
	}

	if err := plugin.HandleNamespaces(sets.NewString("76")); err != nil {
		t.Errorf("TestHandleNamespaces unexpected error after set: %v", err)
	}

	if err := plugin.HandleNamespaces(sets.NewString("76", "711")); err != nil {
		t.Errorf("TestHandleNamespaces unexpected error after set multiple: %v", err)
	}

	if err := plugin.HandleNamespaces(sets.NewString("arco")); err != nil {
		t.Errorf("TestHandleNamespaces unexpected error after reset: %v", err)
	}
}
