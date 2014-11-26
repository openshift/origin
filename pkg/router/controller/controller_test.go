package controller

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
	testrouter "github.com/openshift/origin/pkg/router/test"
)

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

	for name, c := range cases {
		existingFrontends := map[string]router.Frontend{}
		if c.existing {
			existingFrontends[testRouteServiceName] = router.Frontend{}
		}

		testRouter := testrouter.NewRouter(existingFrontends)
		controller := RouterController{
			Router: testRouter,
			NextEndpoints: func() (watch.EventType, *kapi.Endpoints) {
				panic("Unreachable")
			},
			NextRoute: func() (watch.EventType, *routeapi.Route) {
				return c.eventType, &testRoute
			},
		}

		expectedFrontends := 0
		if c.frontendCreated {
			expectedFrontends = 1
		}

		controller.HandleRoute()

		if e, a := expectedFrontends, len(testRouter.CreatedFrontends); e != a {
			t.Errorf("Case %v: Frontend should have been created", name)
		}

		if c.aliasAdded {
			addedAlias, ok := testRouter.AddedAliases[testRouteHost]
			if !ok {
				t.Errorf("Case %v: An alias should have been added for %v", name, testRouteHost)
			}

			if a, e := addedAlias, testRouteServiceName; a != e {
				t.Errorf("Case: %v: Expected added alias for host %v, got %v instead", name, e, a)
			}
		}

		if c.aliasRemoved {
			removedAlias, ok := testRouter.RemovedAliases[testRouteHost]
			if !ok {
				t.Errorf("Case %v: An alias should have been removed for %v", name, testRouteHost)
			}

			if a, e := removedAlias, testRouteServiceName; a != e {
				t.Errorf("Case %v: Expected removed alias for host %v, got %v instead", name, e, a)
			}
		}

		if !testRouter.ConfigWritten {
			t.Errorf("Case %v: Router configs should have been written", name)
		}

		if !testRouter.RouterReloaded {
			t.Errorf("Case %v: Router should have been reloaded", name)
		}
	}
}

func TestHandleEndpointsAdded(t *testing.T) {
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

	for name, c := range cases {
		existingFrontends := map[string]router.Frontend{}
		if c.existing {
			existingFrontends[testEndpointsName] = router.Frontend{}
		}

		testRouter := testrouter.NewRouter(existingFrontends)
		controller := RouterController{
			Router: testRouter,
			NextEndpoints: func() (watch.EventType, *kapi.Endpoints) {
				return c.eventType, &testEndpoints
			},
			NextRoute: func() (watch.EventType, *routeapi.Route) {
				panic("Unreachable")
			},
		}

		expectedFrontends := 0
		if c.frontendCreated {
			expectedFrontends = 1
		}

		controller.HandleEndpoints()

		if e, a := expectedFrontends, len(testRouter.CreatedFrontends); e != a {
			t.Errorf("Case %v: Frontend should have been created", name)
		}

		if len(testRouter.DeletedBackends) != 1 {
			t.Errorf("Case %v: Router should have had one deleted backend", name)
		}

		addedRoutes, ok := testRouter.AddedRoutes[testEndpointsName]
		if c.routesAdded {
			if !ok {
				t.Errorf("Case %v: Two routes should have been added for %v", name, testEndpointsName)
			}

			if num := len(addedRoutes); num != 2 {
				t.Errorf("Case %v: Actual added endpoints %v != 2", name, num)
			}
		} else if ok {
			t.Errorf("Case %v: No routes should have been added for %v", name, testEndpointsName)
		}

		if !testRouter.ConfigWritten {
			t.Errorf("Case %v: Router configs should have been written", name)
		}

		if !testRouter.RouterReloaded {
			t.Errorf("Case %v: Router should have been reloaded", name)
		}
	}
}
