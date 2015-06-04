package templaterouter

import (
	"fmt"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
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
func (r *TestRouter) AddEndpoints(id string, endpoints []Endpoint) {
	r.Committed = false //expect any call to this method to subsequently call commit
	su, _ := r.FindServiceUnit(id)

	for _, ep := range endpoints {
		newEndpoint := Endpoint{ep.ID, ep.IP, ep.Port, "foo"}
		su.EndpointTable = append(su.EndpointTable, newEndpoint)
	}

	r.State[id] = su
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
func (r *TestRouter) AddRoute(id string, route *routeapi.Route) {
	r.Committed = false //expect any call to this method to subsequently call commit
	su, _ := r.FindServiceUnit(id)
	routeKey := r.routeKey(route)

	config := ServiceAliasConfig{
		Host: route.Host,
		Path: route.Path,
	}

	su.ServiceAliasConfigs[routeKey] = config
	r.State[id] = su
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

// routeKey create an identifier for the route consisting of host-path
func (r *TestRouter) routeKey(route *routeapi.Route) string {
	return route.Host + "-" + route.Path
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
	plugin := TemplatePlugin{Router: router}

	for _, tc := range testCases {
		plugin.HandleEndpoints(tc.eventType, tc.endpoints)

		if !router.Committed {
			t.Errorf("Expected router to be committed after HandleEndpoints call")
		}

		su, ok := plugin.Router.FindServiceUnit(tc.expectedServiceUnit.Name)

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
	plugin := TemplatePlugin{Router: router}

	//add
	route := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "foo",
		},
		Host:        "www.example.com",
		ServiceName: "TestService",
	}
	serviceUnitKey := fmt.Sprintf("%s/%s", route.Namespace, route.ServiceName)

	plugin.HandleRoute(watch.Added, route)

	if !router.Committed {
		t.Errorf("Expected router to be committed after HandleRoute call")
	}

	actualSU, ok := router.FindServiceUnit(serviceUnitKey)

	if !ok {
		t.Errorf("TestHandleRoute was unable to find the service unit %s after HandleRoute was called", route.ServiceName)
	} else {
		serviceAliasCfg, ok := actualSU.ServiceAliasConfigs[router.routeKey(route)]

		if !ok {
			t.Errorf("TestHandleRoute expected route key %s", router.routeKey(route))
		} else {
			if serviceAliasCfg.Host != route.Host || serviceAliasCfg.Path != route.Path {
				t.Errorf("Expected route did not match service alias config %v : %v", route, serviceAliasCfg)
			}
		}
	}

	//mod
	route.Host = "www.example2.com"
	plugin.HandleRoute(watch.Modified, route)

	if !router.Committed {
		t.Errorf("Expected router to be committed after HandleRoute call")
	}

	actualSU, ok = router.FindServiceUnit(serviceUnitKey)

	if !ok {
		t.Errorf("TestHandleRoute was unable to find the service unit %s after HandleRoute was called", route.ServiceName)
	} else {
		serviceAliasCfg, ok := actualSU.ServiceAliasConfigs[router.routeKey(route)]

		if !ok {
			t.Errorf("TestHandleRoute expected route key %s", router.routeKey(route))
		} else {
			if serviceAliasCfg.Host != route.Host || serviceAliasCfg.Path != route.Path {
				t.Errorf("Expected route did not match service alias config %v : %v", route, serviceAliasCfg)
			}
		}
	}

	//delete
	plugin.HandleRoute(watch.Deleted, route)

	if !router.Committed {
		t.Errorf("Expected router to be committed after HandleRoute call")
	}

	actualSU, ok = router.FindServiceUnit(serviceUnitKey)

	if !ok {
		t.Errorf("TestHandleRoute was unable to find the service unit %s after HandleRoute was called", route.ServiceName)
	} else {
		_, ok := actualSU.ServiceAliasConfigs[router.routeKey(route)]

		if ok {
			t.Errorf("TestHandleRoute did not expect route key %s", router.routeKey(route))
		}
	}

}
