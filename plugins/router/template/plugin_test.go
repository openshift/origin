package templaterouter

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router/controller"
)

// TestRouter provides an implementation of the plugin's router interface suitable for unit testing.
type TestRouter struct {
	State         map[string]ServiceUnit
	Committed     bool
	ErrorOnCommit error
}

// NewTestRouter creates a new TestRouter and registers the initial state.
func newTestRouter(state map[string]ServiceUnit) *TestRouter {
	return &TestRouter{
		State:         state,
		Committed:     false,
		ErrorOnCommit: nil,
	}
}

// CreateServiceUnit creates an empty service unit identified by id
func (r *TestRouter) CreateServiceUnit(id string) {
	su := ServiceUnit{
		Name:                id,
		ServiceAliasConfigs: make(map[string]ServiceAliasConfig),
		EndpointTable:       []Endpoint{},
	}

	r.State[id] = su
}

// FindServiceUnit finds the service unit in the state
func (r *TestRouter) FindServiceUnit(id string) (v ServiceUnit, ok bool) {
	v, ok = r.State[id]
	return
}

// AddEndpoints adds the endpoints to the service unit identified by id
func (r *TestRouter) AddEndpoints(id string, endpoints []Endpoint) bool {
	r.Committed = false //expect any call to this method to subsequently call commit
	su, _ := r.FindServiceUnit(id)

	// simulate the logic that compares endpoints
	if reflect.DeepEqual(su.EndpointTable, endpoints) {
		return false
	}
	su.EndpointTable = endpoints
	r.State[id] = su
	return true
}

// DeleteEndpoints removes all endpoints from the service unit
func (r *TestRouter) DeleteEndpoints(id string) {
	r.Committed = false //expect any call to this method to subsequently call commit
	if su, ok := r.FindServiceUnit(id); !ok {
		return
	} else {
		su.EndpointTable = []Endpoint{}
		r.State[id] = su
	}
}

// AddRoute adds a ServiceAliasConfig for the route to the ServiceUnit identified by id
func (r *TestRouter) AddRoute(id string, route *routeapi.Route, host string) bool {
	r.Committed = false //expect any call to this method to subsequently call commit
	su, _ := r.FindServiceUnit(id)
	routeKey := r.routeKey(route)

	config := ServiceAliasConfig{
		Host: host,
		Path: route.Spec.Path,
	}

	su.ServiceAliasConfigs[routeKey] = config
	r.State[id] = su
	return true
}

// RemoveRoute removes the service alias config for Route from the ServiceUnit
func (r *TestRouter) RemoveRoute(id string, route *routeapi.Route) {
	r.Committed = false //expect any call to this method to subsequently call commit
	if _, ok := r.State[id]; !ok {
		return
	} else {
		delete(r.State[id].ServiceAliasConfigs, r.routeKey(route))
	}
}

func (r *TestRouter) FilterNamespaces(namespaces sets.String) {
	if len(namespaces) == 0 {
		r.State = make(map[string]ServiceUnit)
	}
	for k := range r.State {
		// TODO: the id of a service unit should be defined inside this class, not passed in from the outside
		//   remove the leak of the abstraction when we refactor this code
		ns := strings.SplitN(k, "/", 2)[0]
		if namespaces.Has(ns) {
			continue
		}
		delete(r.State, k)
	}
}

// routeKey create an identifier for the route consisting of host-path
func (r *TestRouter) routeKey(route *routeapi.Route) string {
	return route.Spec.Host + "-" + route.Spec.Path
}

// Commit saves router state
func (r *TestRouter) Commit() error {
	r.Committed = true
	return r.ErrorOnCommit
}

// TestHandleEndpoints test endpoint watch events
func TestHandleEndpoints(t *testing.T) {
	testCases := []struct {
		name                string          //human readable name for test case
		eventType           watch.EventType //type to be passed to the HandleEndpoints method
		endpoints           *kapi.Endpoints //endpoints to be passed to the HandleEndpoints method
		expectedServiceUnit *ServiceUnit    //service unit that will be compared against.
		excludeUDP          bool
	}{
		{
			name:      "Endpoint add",
			eventType: watch.Added,
			endpoints: &kapi.Endpoints{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "test", //kapi.endpoints inherits the name of the service
				},
				Subsets: []kapi.EndpointSubset{{
					Addresses: []kapi.EndpointAddress{{IP: "1.1.1.1"}},
					Ports:     []kapi.EndpointPort{{Port: 345}},
				}}, //not specifying a port to force the port 80 assumption
			},
			expectedServiceUnit: &ServiceUnit{
				Name: "foo/test", //service name from kapi.endpoints object
				EndpointTable: []Endpoint{
					{
						ID:   "1.1.1.1:345",
						IP:   "1.1.1.1",
						Port: "345",
					},
				},
			},
		},
		{
			name:      "Endpoint mod",
			eventType: watch.Modified,
			endpoints: &kapi.Endpoints{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "test",
				},
				Subsets: []kapi.EndpointSubset{{
					Addresses: []kapi.EndpointAddress{{IP: "2.2.2.2"}},
					Ports:     []kapi.EndpointPort{{Port: 8080}},
				}},
			},
			expectedServiceUnit: &ServiceUnit{
				Name: "foo/test",
				EndpointTable: []Endpoint{
					{
						ID:   "2.2.2.2:8080",
						IP:   "2.2.2.2",
						Port: "8080",
					},
				},
			},
		},
		{
			name:      "Endpoint delete",
			eventType: watch.Deleted,
			endpoints: &kapi.Endpoints{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "test",
				},
				Subsets: []kapi.EndpointSubset{{
					Addresses: []kapi.EndpointAddress{{IP: "3.3.3.3"}},
					Ports:     []kapi.EndpointPort{{Port: 0}},
				}},
			},
			expectedServiceUnit: &ServiceUnit{
				Name:          "foo/test",
				EndpointTable: []Endpoint{},
			},
		},
	}

	router := newTestRouter(make(map[string]ServiceUnit))
	templatePlugin := newDefaultTemplatePlugin(router, true)
	// TODO: move tests that rely on unique hosts to pkg/router/controller and remove them from
	// here
	plugin := controller.NewUniqueHost(templatePlugin, controller.HostForRoute)

	for _, tc := range testCases {
		plugin.HandleEndpoints(tc.eventType, tc.endpoints)

		if !router.Committed {
			t.Errorf("Expected router to be committed after HandleEndpoints call")
		}

		su, ok := router.FindServiceUnit(tc.expectedServiceUnit.Name)

		if !ok {
			t.Errorf("TestHandleEndpoints test case %s failed.  Couldn't find expected service unit with name %s", tc.name, tc.expectedServiceUnit.Name)
		} else {
			for expectedKey, expectedEp := range tc.expectedServiceUnit.EndpointTable {
				actualEp := su.EndpointTable[expectedKey]

				if expectedEp.ID != actualEp.ID || expectedEp.IP != actualEp.IP || expectedEp.Port != actualEp.Port {
					t.Errorf("TestHandleEndpoints test case %s failed.  Expected endpoint didn't match actual endpoint %v : %v", tc.name, expectedEp, actualEp)
				}
			}
		}
	}
}

// TestHandleCPEndpoints test endpoint watch events with UDP excluded
func TestHandleTCPEndpoints(t *testing.T) {
	testCases := []struct {
		name                string          //human readable name for test case
		eventType           watch.EventType //type to be passed to the HandleEndpoints method
		endpoints           *kapi.Endpoints //endpoints to be passed to the HandleEndpoints method
		expectedServiceUnit *ServiceUnit    //service unit that will be compared against.
	}{
		{
			name:      "Endpoint add",
			eventType: watch.Added,
			endpoints: &kapi.Endpoints{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "test", //kapi.endpoints inherits the name of the service
				},
				Subsets: []kapi.EndpointSubset{{
					Addresses: []kapi.EndpointAddress{{IP: "1.1.1.1"}},
					Ports: []kapi.EndpointPort{
						{Port: 345},
						{Port: 346, Protocol: kapi.ProtocolUDP},
					},
				}}, //not specifying a port to force the port 80 assumption
			},
			expectedServiceUnit: &ServiceUnit{
				Name: "foo/test", //service name from kapi.endpoints object
				EndpointTable: []Endpoint{
					{
						ID:   "1.1.1.1:345",
						IP:   "1.1.1.1",
						Port: "345",
					},
				},
			},
		},
		{
			name:      "Endpoint mod",
			eventType: watch.Modified,
			endpoints: &kapi.Endpoints{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "test",
				},
				Subsets: []kapi.EndpointSubset{{
					Addresses: []kapi.EndpointAddress{{IP: "2.2.2.2"}},
					Ports: []kapi.EndpointPort{
						{Port: 8080},
						{Port: 8081, Protocol: kapi.ProtocolUDP},
					},
				}},
			},
			expectedServiceUnit: &ServiceUnit{
				Name: "foo/test",
				EndpointTable: []Endpoint{
					{
						ID:   "2.2.2.2:8080",
						IP:   "2.2.2.2",
						Port: "8080",
					},
				},
			},
		},
		{
			name:      "Endpoint delete",
			eventType: watch.Deleted,
			endpoints: &kapi.Endpoints{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "test",
				},
				Subsets: []kapi.EndpointSubset{{
					Addresses: []kapi.EndpointAddress{{IP: "3.3.3.3"}},
					Ports:     []kapi.EndpointPort{{Port: 0}},
				}},
			},
			expectedServiceUnit: &ServiceUnit{
				Name:          "foo/test",
				EndpointTable: []Endpoint{},
			},
		},
	}

	router := newTestRouter(make(map[string]ServiceUnit))
	templatePlugin := newDefaultTemplatePlugin(router, false)
	// TODO: move tests that rely on unique hosts to pkg/router/controller and remove them from
	// here
	plugin := controller.NewUniqueHost(templatePlugin, controller.HostForRoute)

	for _, tc := range testCases {
		plugin.HandleEndpoints(tc.eventType, tc.endpoints)

		if !router.Committed {
			t.Errorf("Expected router to be committed after HandleEndpoints call")
		}

		su, ok := router.FindServiceUnit(tc.expectedServiceUnit.Name)

		if !ok {
			t.Errorf("TestHandleEndpoints test case %s failed.  Couldn't find expected service unit with name %s", tc.name, tc.expectedServiceUnit.Name)
		} else {
			for expectedKey, expectedEp := range tc.expectedServiceUnit.EndpointTable {
				actualEp := su.EndpointTable[expectedKey]

				if expectedEp.ID != actualEp.ID || expectedEp.IP != actualEp.IP || expectedEp.Port != actualEp.Port {
					t.Errorf("TestHandleEndpoints test case %s failed.  Expected endpoint didn't match actual endpoint %v : %v", tc.name, expectedEp, actualEp)
				}
			}
		}
	}
}

// TestHandleRoute test route watch events
func TestHandleRoute(t *testing.T) {
	router := newTestRouter(make(map[string]ServiceUnit))
	templatePlugin := newDefaultTemplatePlugin(router, true)
	// TODO: move tests that rely on unique hosts to pkg/router/controller and remove them from
	// here
	plugin := controller.NewUniqueHost(templatePlugin, controller.HostForRoute)

	original := unversioned.Time{Time: time.Now()}

	//add
	route := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			CreationTimestamp: original,
			Namespace:         "foo",
			Name:              "test",
		},
		Spec: routeapi.RouteSpec{
			Host: "www.example.com",
			To: kapi.ObjectReference{
				Name: "TestService",
			},
		},
	}
	serviceUnitKey := fmt.Sprintf("%s/%s", route.Namespace, route.Spec.To.Name)

	plugin.HandleRoute(watch.Added, route)

	if !router.Committed {
		t.Errorf("Expected router to be committed after HandleRoute call")
	}

	actualSU, ok := router.FindServiceUnit(serviceUnitKey)

	if !ok {
		t.Errorf("TestHandleRoute was unable to find the service unit %s after HandleRoute was called", route.Spec.To.Name)
	} else {
		serviceAliasCfg, ok := actualSU.ServiceAliasConfigs[router.routeKey(route)]

		if !ok {
			t.Errorf("TestHandleRoute expected route key %s", router.routeKey(route))
		} else {
			if serviceAliasCfg.Host != route.Spec.Host || serviceAliasCfg.Path != route.Spec.Path {
				t.Errorf("Expected route did not match service alias config %v : %v", route, serviceAliasCfg)
			}
		}
	}

	// attempt to add a second route with a newer time, verify it is ignored
	duplicateRoute := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			CreationTimestamp: unversioned.Time{Time: original.Add(time.Hour)},
			Namespace:         "foo",
			Name:              "dupe",
		},
		Spec: routeapi.RouteSpec{
			Host: "www.example.com",
			To: kapi.ObjectReference{
				Name: "TestService2",
			},
		},
	}
	if err := plugin.HandleRoute(watch.Added, duplicateRoute); err == nil {
		t.Fatal("unexpected non-error")
	}
	if _, ok := router.FindServiceUnit("foo/TestService2"); ok {
		t.Fatalf("unexpected second unit: %#v", router)
	}
	if r, ok := plugin.RoutesForHost("www.example.com"); !ok || r[0].Name != "test" {
		t.Fatalf("unexpected claimed routes: %#v", r)
	}

	// attempt to remove the second route that is not being used, verify it is ignored
	if err := plugin.HandleRoute(watch.Deleted, duplicateRoute); err == nil {
		t.Fatal("unexpected non-error")
	}
	if _, ok := router.FindServiceUnit("foo/TestService2"); ok {
		t.Fatalf("unexpected second unit: %#v", router)
	}
	if _, ok := router.FindServiceUnit("foo/TestService"); !ok {
		t.Fatalf("unexpected first unit: %#v", router)
	}
	if r, ok := plugin.RoutesForHost("www.example.com"); !ok || r[0].Name != "test" {
		t.Fatalf("unexpected claimed routes: %#v", r)
	}

	// add a second route with an older time, verify it takes effect
	duplicateRoute.CreationTimestamp = unversioned.Time{Time: original.Add(-time.Hour)}
	if err := plugin.HandleRoute(watch.Added, duplicateRoute); err != nil {
		t.Fatal("unexpected error")
	}
	otherSU, ok := router.FindServiceUnit("foo/TestService2")
	if !ok {
		t.Fatalf("missing second unit: %#v", router)
	}
	if len(actualSU.ServiceAliasConfigs) != 0 || len(otherSU.ServiceAliasConfigs) != 1 {
		t.Errorf("incorrect router state: %#v", router)
	}
	if _, ok := actualSU.ServiceAliasConfigs[router.routeKey(route)]; ok {
		t.Errorf("unexpected service alias config %s", router.routeKey(route))
	}

	//mod
	route.Spec.Host = "www.example2.com"
	if err := plugin.HandleRoute(watch.Modified, route); err != nil {
		t.Fatal("unexpected error")
	}
	if !router.Committed {
		t.Errorf("Expected router to be committed after HandleRoute call")
	}
	actualSU, ok = router.FindServiceUnit(serviceUnitKey)
	if !ok {
		t.Errorf("TestHandleRoute was unable to find the service unit %s after HandleRoute was called", route.Spec.To.Name)
	} else {
		serviceAliasCfg, ok := actualSU.ServiceAliasConfigs[router.routeKey(route)]

		if !ok {
			t.Errorf("TestHandleRoute expected route key %s", router.routeKey(route))
		} else {
			if serviceAliasCfg.Host != route.Spec.Host || serviceAliasCfg.Path != route.Spec.Path {
				t.Errorf("Expected route did not match service alias config %v : %v", route, serviceAliasCfg)
			}
		}
	}
	if plugin.HostLen() != 1 {
		t.Fatalf("did not clear claimed route: %#v", plugin)
	}

	//delete
	if err := plugin.HandleRoute(watch.Deleted, route); err != nil {
		t.Fatal("unexpected error")
	}
	if !router.Committed {
		t.Errorf("Expected router to be committed after HandleRoute call")
	}
	actualSU, ok = router.FindServiceUnit(serviceUnitKey)
	if !ok {
		t.Errorf("TestHandleRoute was unable to find the service unit %s after HandleRoute was called", route.Spec.To.Name)
	} else {
		_, ok := actualSU.ServiceAliasConfigs[router.routeKey(route)]

		if ok {
			t.Errorf("TestHandleRoute did not expect route key %s", router.routeKey(route))
		}
	}
	if plugin.HostLen() != 0 {
		t.Errorf("did not clear claimed route: %#v", plugin)
	}
}

func TestNamespaceScopingFromEmpty(t *testing.T) {
	router := newTestRouter(make(map[string]ServiceUnit))
	templatePlugin := newDefaultTemplatePlugin(router, true)
	// TODO: move tests that rely on unique hosts to pkg/router/controller and remove them from
	// here
	plugin := controller.NewUniqueHost(templatePlugin, controller.HostForRoute)

	// no namespaces allowed
	plugin.HandleNamespaces(sets.String{})

	//add
	route := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{Namespace: "foo", Name: "test"},
		Spec: routeapi.RouteSpec{
			Host: "www.example.com",
			To: kapi.ObjectReference{
				Name: "TestService",
			},
		},
	}

	// ignores all events for namespace that doesn't match
	for _, s := range []watch.EventType{watch.Added, watch.Modified, watch.Deleted} {
		plugin.HandleRoute(s, route)
		if _, ok := router.FindServiceUnit("foo/TestService"); ok || plugin.HostLen() != 0 {
			t.Errorf("unexpected router state %#v", router)
		}
	}

	// allow non matching
	plugin.HandleNamespaces(sets.NewString("bar"))
	for _, s := range []watch.EventType{watch.Added, watch.Modified, watch.Deleted} {
		plugin.HandleRoute(s, route)
		if _, ok := router.FindServiceUnit("foo/TestService"); ok || plugin.HostLen() != 0 {
			t.Errorf("unexpected router state %#v", router)
		}
	}

	// allow foo
	plugin.HandleNamespaces(sets.NewString("foo", "bar"))
	plugin.HandleRoute(watch.Added, route)
	if _, ok := router.FindServiceUnit("foo/TestService"); !ok || plugin.HostLen() != 1 {
		t.Errorf("unexpected router state %#v", router)
	}

	// forbid foo, and make sure it's cleared
	plugin.HandleNamespaces(sets.NewString("bar"))
	if _, ok := router.FindServiceUnit("foo/TestService"); ok || plugin.HostLen() != 0 {
		t.Errorf("unexpected router state %#v", router)
	}
	plugin.HandleRoute(watch.Modified, route)
	if _, ok := router.FindServiceUnit("foo/TestService"); ok || plugin.HostLen() != 0 {
		t.Errorf("unexpected router state %#v", router)
	}
	plugin.HandleRoute(watch.Added, route)
	if _, ok := router.FindServiceUnit("foo/TestService"); ok || plugin.HostLen() != 0 {
		t.Errorf("unexpected router state %#v", router)
	}
}

func TestUnchangingEndpointsDoesNotCommit(t *testing.T) {
	router := newTestRouter(make(map[string]ServiceUnit))
	plugin := newDefaultTemplatePlugin(router, true)
	endpoints := &kapi.Endpoints{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "foo",
			Name:      "test",
		},
		Subsets: []kapi.EndpointSubset{{
			Addresses: []kapi.EndpointAddress{{IP: "1.1.1.1"}, {IP: "2.2.2.2"}},
			Ports:     []kapi.EndpointPort{{Port: 0}},
		}},
	}
	changedEndpoints := &kapi.Endpoints{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "foo",
			Name:      "test",
		},
		Subsets: []kapi.EndpointSubset{{
			Addresses: []kapi.EndpointAddress{{IP: "3.3.3.3"}, {IP: "2.2.2.2"}},
			Ports:     []kapi.EndpointPort{{Port: 0}},
		}},
	}

	testCases := []struct {
		name         string
		event        watch.EventType
		endpoints    *kapi.Endpoints
		expectCommit bool
	}{
		{
			name:         "initial add",
			event:        watch.Added,
			endpoints:    endpoints,
			expectCommit: true,
		},
		{
			name:         "mod with no change",
			event:        watch.Modified,
			endpoints:    endpoints,
			expectCommit: false,
		},
		{
			name:         "add with change",
			event:        watch.Added,
			endpoints:    changedEndpoints,
			expectCommit: true,
		},
		{
			name:         "add with no change",
			event:        watch.Added,
			endpoints:    changedEndpoints,
			expectCommit: false,
		},
	}

	for _, v := range testCases {
		err := plugin.HandleEndpoints(v.event, v.endpoints)
		if err != nil {
			t.Errorf("%s had unexpected error in handle endpoints %v", v.name, err)
			continue
		}
		if router.Committed != v.expectCommit {
			t.Errorf("%s expected router commit to be %v but found %v", v.name, v.expectCommit, router.Committed)
		}
	}
}
