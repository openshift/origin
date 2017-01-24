package integration

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/router/controller"
	controllerfactory "github.com/openshift/origin/pkg/router/controller/factory"
	templateplugin "github.com/openshift/origin/pkg/router/template"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

// TestRouterReloadSuppressionOnSync validates that the router will
// not be reloaded until all events from the initial sync have been
// processed.  Reload should similarly suppressed on subsequent
// resyncs.
func TestRouterReloadSuppressionOnSync(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	stressRouter(
		t,
		// Allow the test to be configured to enable experimentation
		// without a costly (~60s+) go build.
		cmdutil.EnvInt("OS_TEST_NAMESPACE_COUNT", 1, 1),
		cmdutil.EnvInt("OS_TEST_ROUTES_PER_NAMESPACE", 5, 5),
		cmdutil.EnvInt("OS_TEST_ROUTER_COUNT", 1, 1),
		cmdutil.EnvInt("OS_TEST_MAX_ROUTER_DELAY", 10, 10),
	)
}

func stressRouter(t *testing.T, namespaceCount, routesPerNamespace, routerCount, maxRouterDelay int32) {
	testutil.RequireEtcd(t)

	oc, kc, err := launchApi()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Keep track of created routes to be able to verify against
	// the processed router state.
	routes := []*routeapi.Route{}

	// Create initial state
	for i := int32(0); i < namespaceCount; i++ {

		// Create a namespace
		namespaceProperties := createNamespaceProperties()
		namespace, err := kc.Core().Namespaces().Create(namespaceProperties)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for j := int32(0); j < routesPerNamespace; j++ {

			// Create a service for the route
			serviceProperties := createServiceProperties()
			service, err := kc.Core().Services(namespace.Name).Create(serviceProperties)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Create endpoints
			endpointsProperties := createEndpointsProperties(service.Name)
			_, err = kc.Core().Endpoints(namespace.Name).Create(endpointsProperties)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Create a route
			host := fmt.Sprintf("www-%d-%d.example.com", i, j)
			routeProperties := createRouteProperties(service.Name, host)
			route, err := oc.Routes(namespace.Name).Create(routeProperties)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			routes = append(routes, route)

		}
		// Create a final route that uses the same host as the
		// previous route to create a conflict.  This will validate
		// that a router still reloads if the last item in the initial
		// list is rejected.
		host := routes[len(routes)-1].Spec.Host
		routeProperties := createRouteProperties("invalid-service", host)
		_, err = oc.Routes(namespace.Name).Create(routeProperties)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Keep track of the plugins to allow access to the router state
	// after processing.
	plugins := []*templateplugin.TemplatePlugin{}

	// Don't coalesce reloads to validate reload suppression during sync.
	reloadInterval := 0

	// Track reload counts indexed by router name.
	reloadedMap := make(map[string]int)

	// Create the routers
	for i := int32(0); i < routerCount; i++ {
		routerName := fmt.Sprintf("router_%d", i)
		router := launchRouter(oc, kc, maxRouterDelay, routerName, reloadInterval, reloadedMap)
		plugins = append(plugins, router)
	}

	// Wait for the routers to reload
	for {
		allReloaded := true
		for _, reloadCount := range reloadedMap {
			if reloadCount == 0 {
				allReloaded = false
				break
			}
		}
		if allReloaded {
			break
		} else {
			time.Sleep(time.Second)
		}
	}

	// Check that the routers have processed all routes.  The delay
	// plugin should ensure that router state reflects only the
	// initial sync and not subsequent watch events.
	expectedRouteCount := len(routes)
	for _, plugin := range plugins {
		routeCount := 0
		for _, route := range routes {
			if plugin.Router.HasRoute(route) {
				routeCount++
			}
		}
		if routeCount != expectedRouteCount {
			t.Fatalf("Expected %v routes, got %v", expectedRouteCount, routeCount)
		}
	}

	for _, reloadCount := range reloadedMap {
		if reloadCount > 1 {
			// If a router reloads more than once, post-sync watch
			// events resulting from route status updates are
			// incorrectly updating router state.
			t.Fatalf("One or more routers reloaded more than once")
		}
	}

}

func createNamespaceProperties() *kapi.Namespace {
	return &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			GenerateName: "namespace-",
		},
		Status: kapi.NamespaceStatus{},
	}
}

func createServiceProperties() *kapi.Service {
	return &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			GenerateName: "service-",
		},
		Spec: kapi.ServiceSpec{
			Ports: []kapi.ServicePort{{
				Protocol: "TCP",
				Port:     8080,
			}},
		},
	}
}

func createEndpointsProperties(serviceName string) *kapi.Endpoints {
	return &kapi.Endpoints{
		ObjectMeta: kapi.ObjectMeta{
			Name: serviceName,
		},
		Subsets: []kapi.EndpointSubset{{
			Addresses: []kapi.EndpointAddress{{
				IP: "1.2.3.4",
			}},
			Ports: []kapi.EndpointPort{{
				Port: 8080,
			}},
		}},
	}
}

func createRouteProperties(serviceName, host string) *routeapi.Route {
	return &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			GenerateName: "route-",
		},
		Spec: routeapi.RouteSpec{
			Host: host,
			Path: "",
			To: routeapi.RouteTargetReference{
				Name: serviceName,
			},
			TLS: nil,
		},
	}
}

// launchAPI launches an api server and returns clients configured to
// access it.
func launchApi() (osclient.Interface, kclientset.Interface, error) {
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		return nil, nil, err
	}

	kc, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		return nil, nil, err
	}

	oc, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		return nil, nil, err
	}

	return oc, kc, nil
}

// DelayPlugin implements the router.Plugin interface to introduce
// random delay into plugin handlers to simulate variation in
// processing time when a router is under load.
type DelayPlugin struct {
	plugin   router.Plugin
	maxDelay int32
}

func NewDelayPlugin(plugin router.Plugin, maxDelay int32) *DelayPlugin {
	rand.Seed(time.Now().UTC().UnixNano())
	return &DelayPlugin{
		plugin:   plugin,
		maxDelay: maxDelay,
	}
}

func (p *DelayPlugin) delay() {
	time.Sleep(time.Duration(rand.Int31n(p.maxDelay)) * time.Millisecond)
}

func (p *DelayPlugin) HandleRoute(eventType watch.EventType, route *routeapi.Route) error {
	p.delay()
	return p.plugin.HandleRoute(eventType, route)
}

func (p *DelayPlugin) HandleNode(eventType watch.EventType, node *kapi.Node) error {
	p.delay()
	return p.plugin.HandleNode(eventType, node)
}

func (p *DelayPlugin) HandleEndpoints(eventType watch.EventType, endpoints *kapi.Endpoints) error {
	p.delay()
	return p.plugin.HandleEndpoints(eventType, endpoints)
}

func (p *DelayPlugin) HandleNamespaces(namespaces sets.String) error {
	p.delay()
	return p.plugin.HandleNamespaces(namespaces)
}

func (p *DelayPlugin) Commit() error {
	return p.plugin.Commit()
}

// launchRouter launches a template router that communicates with the
// api via the provided clients.
func launchRouter(oc osclient.Interface, kc kclientset.Interface, maxDelay int32, name string, reloadInterval int, reloadedMap map[string]int) (templatePlugin *templateplugin.TemplatePlugin) {
	r := templateplugin.NewFakeTemplateRouter()

	reloadedMap[name] = 0
	r.EnableRateLimiter(reloadInterval, func() error {
		reloadedMap[name] += 1
		return nil
	})

	templatePlugin = &templateplugin.TemplatePlugin{Router: r}

	statusPlugin := controller.NewStatusAdmitter(templatePlugin, oc, name, "")

	validationPlugin := controller.NewExtendedValidator(statusPlugin, controller.RejectionRecorder(statusPlugin))

	uniquePlugin := controller.NewUniqueHost(validationPlugin, controller.HostForRoute, false, controller.RejectionRecorder(statusPlugin))

	var plugin router.Plugin = uniquePlugin
	if maxDelay > 0 {
		plugin = NewDelayPlugin(plugin, maxDelay)
	}

	factory := controllerfactory.NewDefaultRouterControllerFactory(oc, kc)
	ctrl := factory.Create(plugin, false, false)
	ctrl.Run()

	return
}
