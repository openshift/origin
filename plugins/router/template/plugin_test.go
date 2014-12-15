package templaterouter

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
	"reflect"
)

// TestRouter provides an implementation of the plugin's router interface
// suitable for unit testing.
type TestRouter struct {
	FrontendsToFind map[string]Frontend
	ErrorOnCommit   error

	DeletedBackends  []string
	CreatedFrontends []string
	DeletedFrontends []string
	AddedAliases     map[string]string
	RemovedAliases   map[string]string
	AddedRoutes      map[string][]Endpoint
	Commited         bool
}

// NewTestRouter creates a new TestRouter.
func newTestRouter(registeredFrontends map[string]Frontend) *TestRouter {
	return &TestRouter{
		FrontendsToFind:  registeredFrontends,
		DeletedBackends:  []string{},
		CreatedFrontends: []string{},
		DeletedFrontends: []string{},
		AddedAliases:     map[string]string{},
		RemovedAliases:   map[string]string{},
		AddedRoutes:      map[string][]Endpoint{},
	}
}

func (r *TestRouter) FindFrontend(name string) (Frontend, bool) {
	f, ok := r.FrontendsToFind[name]
	return f, ok
}

func (r *TestRouter) DeleteBackends(name string) {
	r.DeletedBackends = append(r.DeletedBackends, name)
}

func (r *TestRouter) CreateFrontend(name, url string) {
	r.CreatedFrontends = append(r.CreatedFrontends, name)
}

func (r *TestRouter) DeleteFrontend(name string) {
	r.DeletedFrontends = append(r.DeletedFrontends, name)
}

func (r *TestRouter) AddAlias(name, alias string) {
	r.AddedAliases[alias] = name
}

func (r *TestRouter) RemoveAlias(name, alias string) {
	r.RemovedAliases[alias] = name
}

func (r *TestRouter) AddRoute(name string, backend *Backend, endpoints []Endpoint) {
	r.AddedRoutes[name] = endpoints
}

func (r *TestRouter) Commit() error {
	r.Commited = true

	return r.ErrorOnCommit
}

func TestHandleRoute(t *testing.T) {
	var (
		testRouteName        = "testroute"
		testRouteServiceName = "testservice"
		testRouteHost        = "test.com"
		testRoute            = routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name: testRouteName,
			},
			Host:        testRouteHost,
			ServiceName: testRouteServiceName,
		}
	)

	cases := map[string]struct {
		eventType       watch.EventType
		existing        bool
		frontendCreated bool
		aliasAdded      bool
		aliasRemoved    bool
	}{
		"added":    {eventType: watch.Added, frontendCreated: true, aliasAdded: true},
		"modified": {eventType: watch.Modified, existing: true, aliasAdded: true},
		"deleted":  {eventType: watch.Deleted, existing: true, aliasRemoved: true},
	}

	for name, tc := range cases {
		existingFrontends := map[string]Frontend{}
		if tc.existing {
			existingFrontends[testRouteServiceName] = Frontend{}
		}

		testRouter := newTestRouter(existingFrontends)
		plugin := TemplatePlugin{
			Router: testRouter,
		}

		expectedFrontends := 0
		if tc.frontendCreated {
			expectedFrontends = 1
		}

		plugin.HandleRoute(tc.eventType, &testRoute)

		if e, a := expectedFrontends, len(testRouter.CreatedFrontends); e != a {
			t.Errorf("Case %v: Frontend should have been created", name)
		}

		if tc.aliasAdded {
			addedAlias, ok := testRouter.AddedAliases[testRouteHost]
			if !ok {
				t.Errorf("Case %v: An alias should have been added for %v", name, testRouteHost)
			}

			if a, e := addedAlias, testRouteServiceName; a != e {
				t.Errorf("Case: %v: Expected added alias for host %v, got %v instead", name, e, a)
			}
		}

		if tc.aliasRemoved {
			removedAlias, ok := testRouter.RemovedAliases[testRouteHost]
			if !ok {
				t.Errorf("Case %v: An alias should have been removed for %v", name, testRouteHost)
			}

			if a, e := removedAlias, testRouteServiceName; a != e {
				t.Errorf("Case %v: Expected removed alias for host %v, got %v instead", name, e, a)
			}
		}

		if !testRouter.Commited {
			t.Errorf("Case %v: Router changes should have been committed", name)
		}
	}
}

func TestHandleEndpoints(t *testing.T) {
	var (
		testEndpointsName = "testendpoints"
		testEndpoints     = kapi.Endpoints{
			ObjectMeta: kapi.ObjectMeta{
				Name: testEndpointsName,
			},
			Endpoints: []string{
				"test1.com:8080",
				"test2.com",
			},
		}
	)

	cases := map[string]struct {
		eventType       watch.EventType
		existing        bool
		frontendCreated bool
		routesAdded     bool
	}{
		"added":    {eventType: watch.Added, frontendCreated: true, routesAdded: true},
		"modified": {eventType: watch.Modified, existing: true, routesAdded: true},
		"deleted":  {eventType: watch.Deleted, existing: true},
	}

	for name, tc := range cases {
		existingFrontends := map[string]Frontend{}
		if tc.existing {
			existingFrontends[testEndpointsName] = Frontend{}
		}

		testRouter := newTestRouter(existingFrontends)
		plugin := TemplatePlugin{
			Router: testRouter,
		}

		expectedFrontends := 0
		if tc.frontendCreated {
			expectedFrontends = 1
		}

		plugin.HandleEndpoints(tc.eventType, &testEndpoints)

		if e, a := expectedFrontends, len(testRouter.CreatedFrontends); e != a {
			t.Errorf("Case %v: Frontend should have been created", name)
		}

		if len(testRouter.DeletedBackends) != 1 {
			t.Errorf("Case %v: Router should have had one deleted backend", name)
		}

		addedRoutes, ok := testRouter.AddedRoutes[testEndpointsName]
		if tc.routesAdded {
			if !ok {
				t.Errorf("Case %v: Two routes should have been added for %v", name, testEndpointsName)
			}

			if num := len(addedRoutes); num != 2 {
				t.Errorf("Case %v: Actual added endpoints %v != 2", name, num)
			}
		} else if ok {
			t.Errorf("Case %v: No routes should have been added for %v", name, testEndpointsName)
		}

		if !testRouter.Commited {
			t.Errorf("Case %v: Router changes should have been committed", name)
		}
	}
}

//test creation of endpoint from a string
func TestEndpointFromString(t *testing.T) {
	endpointFromStringTestCases := map[string]struct {
		InputEndpoint    string
		ExpectedEndpoint *Endpoint
		ExpectedOk       bool
	}{
		"Empty String": {
			InputEndpoint:    "",
			ExpectedEndpoint: nil,
			ExpectedOk:       false,
		},
		"Default Port": {
			InputEndpoint: "test",
			ExpectedEndpoint: &Endpoint{
				IP:   "test",
				Port: "80",
			},
			ExpectedOk: true,
		},
		"Non-default Port": {
			InputEndpoint: "test:9999",
			ExpectedEndpoint: &Endpoint{
				IP:   "test",
				Port: "9999",
			},
			ExpectedOk: true,
		},
	}

	for k, tc := range endpointFromStringTestCases {
		endpoint, ok := endpointFromString(tc.InputEndpoint)

		if ok != tc.ExpectedOk {
			t.Fatalf("%s failed, expected ok=%t but got %t", k, tc.ExpectedOk, ok)
		}

		if !reflect.DeepEqual(endpoint, tc.ExpectedEndpoint) {
			t.Fatalf("%s failed, the returned endpoint didn't match the expected endpoint", k)
		}
	}
}
