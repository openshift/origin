package controller

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
	testrouter "github.com/openshift/origin/pkg/router/controller/test"
)

func TestHandleRouteAdded(t *testing.T) {
	var (
		testRouteName        = "testroute"
		testRouteServiceName = "testservice"
		testRouteHost        = "test.com"
		testRouter           = testrouter.NewRouter(nil)
		testRoute            = routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name: testRouteName,
			},
			Host:        testRouteHost,
			ServiceName: testRouteServiceName,
		}
	)

	controller := RouterController{
		Router: testRouter,
		NextEndpoints: func() (watch.EventType, *kapi.Endpoints) {
			panic("Unreachable")
		},
		NextRoute: func() (watch.EventType, *routeapi.Route) {
			return watch.Added, &testRoute
		},
	}

	controller.HandleRoute()

	if len(testRouter.CreatedFrontends) != 1 {
		t.Fatal("Router should have had one frontend created")
	}

	addedAlias, ok := testRouter.AddedAliases[testRouteHost]
	if !ok {
		t.Fatalf("An alias should have been added for %v", testRouteHost)
	}

	if a, e := addedAlias, testRouteServiceName; a != e {
		t.Fatalf("Expected added alias for host %v, got %v instead", e, a)
	}

	if !testRouter.RouterReloaded {
		t.Fatal("Router should have been reloaded")
	}
}

func TestHandleRouteModified(t *testing.T) {
	var (
		testRouteName        = "testroute"
		testRouteServiceName = "testservice"
		testRouteHost        = "test.com"
		testRouter           = testrouter.NewRouter(map[string]router.Frontend{
			testRouteServiceName: {},
		})
		testRoute = routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name: testRouteName,
			},
			Host:        testRouteHost,
			ServiceName: testRouteServiceName,
		}
	)

	controller := RouterController{
		Router: testRouter,
		NextEndpoints: func() (watch.EventType, *kapi.Endpoints) {
			panic("Unreachable")
		},
		NextRoute: func() (watch.EventType, *routeapi.Route) {
			return watch.Modified, &testRoute
		},
	}

	controller.HandleRoute()

	if len(testRouter.CreatedFrontends) != 0 {
		t.Fatal("No router frontends should have been created")
	}

	addedAlias, ok := testRouter.AddedAliases[testRouteHost]
	if !ok {
		t.Fatalf("An alias should have been added for %v", testRouteHost)
	}

	if a, e := addedAlias, testRouteServiceName; a != e {
		t.Fatalf("Expected added alias for host %v, got %v instead", e, a)
	}

	if !testRouter.RouterReloaded {
		t.Fatal("Router should have been reloaded")
	}
}

func TestHandleRouteDeleted(t *testing.T) {
	var (
		testRouteName        = "testroute"
		testRouteServiceName = "testservice"
		testRouteHost        = "test.com"
		testRouter           = testrouter.NewRouter(map[string]router.Frontend{
			testRouteServiceName: {},
		})
		testRoute = routeapi.Route{
			ObjectMeta: kapi.ObjectMeta{
				Name: testRouteName,
			},
			Host:        testRouteHost,
			ServiceName: testRouteServiceName,
		}
	)

	controller := RouterController{
		Router: testRouter,
		NextEndpoints: func() (watch.EventType, *kapi.Endpoints) {
			panic("Unreachable")
		},
		NextRoute: func() (watch.EventType, *routeapi.Route) {
			return watch.Deleted, &testRoute
		},
	}

	controller.HandleRoute()

	if len(testRouter.CreatedFrontends) != 0 {
		t.Fatal("No router frontends should have been created")
	}

	removedAlias, ok := testRouter.RemovedAliases[testRouteHost]
	if !ok {
		t.Fatalf("An alias should have been added for %v", testRouteHost)
	}

	if a, e := removedAlias, testRouteServiceName; a != e {
		t.Fatalf("Expected removed alias for host %v, got %v instead", e, a)
	}

	if !testRouter.RouterReloaded {
		t.Fatal("Router should have been reloaded")
	}
}

func TestHandleEndpointsAdded(t *testing.T) {
	var (
		testEndpointsName = "testendpoints"
		testRouter        = testrouter.NewRouter(nil)
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

	controller := RouterController{
		Router: testRouter,
		NextEndpoints: func() (watch.EventType, *kapi.Endpoints) {
			return watch.Added, &testEndpoints
		},
		NextRoute: func() (watch.EventType, *routeapi.Route) {
			panic("Unreachable")
		},
	}

	controller.HandleEndpoints()

	if len(testRouter.CreatedFrontends) != 1 {
		t.Fatal("Router should have had one frontend created")
	}

	if len(testRouter.DeletedBackends) != 1 {
		t.Fatal("Router should have had one deleted backend")
	}

	if !testRouter.ConfigWritten {
		t.Fatal("Router configs should have been written")
	}

	if !testRouter.RouterReloaded {
		t.Fatal("Router should have been reloaded")
	}

	addedRoutes, ok := testRouter.AddedRoutes[testEndpointsName]
	if !ok {
		t.Fatalf("Two routes should have been added for %v", testEndpointsName)
	}

	if num := len(addedRoutes); num != 2 {
		t.Fatalf("Actual added endpoints %v != 2", num)
	}
}

func TestHandleEndpointsModified(t *testing.T) {
	var (
		testEndpointsName = "testendpoints"
		testRouter        = testrouter.NewRouter(map[string]router.Frontend{
			testEndpointsName: {},
		})
		testEndpoints = kapi.Endpoints{
			ObjectMeta: kapi.ObjectMeta{
				Name: testEndpointsName,
			},
			Endpoints: []string{
				"test1.com:8080",
				"test2.com",
			},
		}
	)

	controller := RouterController{
		Router: testRouter,
		NextEndpoints: func() (watch.EventType, *kapi.Endpoints) {
			return watch.Modified, &testEndpoints
		},
		NextRoute: func() (watch.EventType, *routeapi.Route) {
			panic("Unreachable")
		},
	}

	controller.HandleEndpoints()

	if len(testRouter.CreatedFrontends) != 0 {
		t.Fatal("Router should have had one frontend created")
	}

	if len(testRouter.DeletedBackends) != 1 {
		t.Fatal("Router should have had one deleted backend")
	}

	if !testRouter.ConfigWritten {
		t.Fatal("Router configs should have been written")
	}

	if !testRouter.RouterReloaded {
		t.Fatal("Router should have been reloaded")
	}

	addedRoutes, ok := testRouter.AddedRoutes[testEndpointsName]
	if !ok {
		t.Fatalf("Two routes should have been added for %v", testEndpointsName)
	}

	if num := len(addedRoutes); num != 2 {
		t.Fatalf("Actual added endpoints %v != 2", num)
	}
}

func TestHandleEndpointsDeleted(t *testing.T) {
	var (
		testEndpointsName = "testendpoints"
		testRouter        = testrouter.NewRouter(map[string]router.Frontend{
			testEndpointsName: {
				Name: testEndpointsName,
			},
		})
		testEndpoints = kapi.Endpoints{
			ObjectMeta: kapi.ObjectMeta{
				Name: testEndpointsName,
			},
			Endpoints: []string{
				"test1.com:8080",
				"test2.com",
			},
		}
	)

	controller := RouterController{
		Router: testRouter,
		NextEndpoints: func() (watch.EventType, *kapi.Endpoints) {
			return watch.Deleted, &testEndpoints
		},
		NextRoute: func() (watch.EventType, *routeapi.Route) {
			panic("Unreachable")
		},
	}

	controller.HandleEndpoints()

	if len(testRouter.CreatedFrontends) != 0 {
		t.Fatal("No frontends should have been created")
	}

	if len(testRouter.DeletedBackends) != 1 {
		t.Fatal("Router should have had one deleted backend")
	}

	if !testRouter.ConfigWritten {
		t.Fatal("Router configs should have been written")
	}

	if !testRouter.RouterReloaded {
		t.Fatal("Router should have been reloaded")
	}

	_, ok := testRouter.AddedRoutes[testEndpointsName]
	if ok {
		t.Fatalf("No routes should have been added for %v", testEndpointsName)
	}
}
