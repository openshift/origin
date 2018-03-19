package integration

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	infrarouter "github.com/openshift/origin/pkg/cmd/infra/router"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	projectinternalclientset "github.com/openshift/origin/pkg/project/generated/internalclientset"
	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	routeinternalclientset "github.com/openshift/origin/pkg/route/generated/internalclientset"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/router/controller"
	controllerfactory "github.com/openshift/origin/pkg/router/controller/factory"
	templateplugin "github.com/openshift/origin/pkg/router/template"
	"github.com/openshift/origin/pkg/util/ratelimiter"
	"github.com/openshift/origin/pkg/util/writerlease"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

const waitInterval = 50 * time.Millisecond

// TestRouterNamespaceSync validates that labeling a namespace so it
// will match a router's selector will expose routes in that namespace
// after the subsequent namespace sync.
func TestRouterNamespaceSync(t *testing.T) {
	routeclient, projectclient, kc, fn, err := launchApi(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer fn()

	// Create a route in a namespace without the label
	namespace := createNamespace(t, kc)
	host := "www.example.com"
	route := initializeNewRoute(t, routeclient, kc, namespace.Name, host)

	// Create a router that filters by the namespace label and with a low reysnc interval
	labelKey := "foo"
	labelValue := "bar"
	selector, err := labels.Parse(fmt.Sprintf("%s=%s", labelKey, labelValue))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	routerSelection := infrarouter.RouterSelection{
		NamespaceLabels: selector,
		ResyncInterval:  2 * time.Second,
	}
	templatePlugin := launchRouter(routeclient, projectclient, kc, routerSelection)

	// Wait until the router has completed initial sync
	err = wait.PollImmediate(waitInterval, wait.ForeverTestTimeout, func() (bool, error) {
		if templatePlugin.Router.SyncedAtLeastOnce() {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The route should not appear
	if templatePlugin.Router.HasRoute(route) {
		t.Fatalf("Route in unselected namespace is unexpectedly exposed on router")
	}

	// Label the namespace
	updateNamespaceLabels(t, kc, namespace, map[string]string{labelKey: labelValue})

	// Wait for the route to appear
	waitForRouterToHaveRoute(t, templatePlugin, route, true)

	// Remove the label from the namespace
	updateNamespaceLabels(t, kc, namespace, map[string]string{})

	// Wait for the route to disappear
	waitForRouterToHaveRoute(t, templatePlugin, route, false)
}

// TestRouterFirstReloadSuppressionOnSync validates that the router will not be reloaded
// until all events from the initial sync have been processed.
func TestRouterFirstReloadSuppressionOnSync(t *testing.T) {
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
	routeclient, projectclient, kc, fn, err := launchApi(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer fn()

	// Keep track of created routes to be able to verify against
	// the processed router state.
	routes := []*routeapi.Route{}

	// Create initial state
	for i := int32(0); i < namespaceCount; i++ {

		namespace := createNamespace(t, kc)

		for j := int32(0); j < routesPerNamespace; j++ {
			host := fmt.Sprintf("www-%d-%d.example.com", i, j)
			route := initializeNewRoute(t, routeclient, kc, namespace.Name, host)
			routes = append(routes, route)
		}
		// Create a final route that uses the same host as the
		// previous route to create a conflict.  This will validate
		// that a router still reloads if the last item in the initial
		// list is rejected.
		host := routes[len(routes)-1].Spec.Host
		routeProperties := createRouteProperties("invalid-service", host)
		_, err = routeclient.Route().Routes(namespace.Name).Create(routeProperties)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Keep track of the plugins to allow access to the router state
	// after processing.
	plugins := []*templateplugin.TemplatePlugin{}

	// Don't coalesce reloads to validate reload suppression during sync.
	reloadInterval := 0 * time.Second

	// Track reload counts indexed by router name.
	reloadedMap := make(map[string]int)

	// Create the routers
	for i := int32(0); i < routerCount; i++ {
		routerName := fmt.Sprintf("router_%d", i)
		router := launchRateLimitedRouter(t, routeclient, projectclient, kc, routerName, maxRouterDelay, reloadInterval, reloadedMap)
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

	for routerName, reloadCount := range reloadedMap {
		if reloadCount > 1 {
			// If a router reloads more than once, post-sync watch
			// events resulting from route status updates are
			// incorrectly updating router state.
			t.Fatalf("One or more routers reloaded more than once (%s reloaded %d times)", routerName, reloadCount)
		}
	}

}

func createNamespace(t *testing.T, kc kclientset.Interface) *kapi.Namespace {
	namespaceProperties := createNamespaceProperties()
	namespace, err := kc.Core().Namespaces().Create(namespaceProperties)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return namespace
}

func initializeNewRoute(t *testing.T, routeclient routeinternalclientset.Interface, kc kclientset.Interface, nsName, host string) *routeapi.Route {
	// Create a service for the route
	serviceProperties := createServiceProperties()
	service, err := kc.Core().Services(nsName).Create(serviceProperties)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create endpoints
	endpointsProperties := createEndpointsProperties(service.Name)
	_, err = kc.Core().Endpoints(nsName).Create(endpointsProperties)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a route
	routeProperties := createRouteProperties(service.Name, host)
	route, err := routeclient.Route().Routes(nsName).Create(routeProperties)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return route
}

func createNamespaceProperties() *kapi.Namespace {
	return &kapi.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "namespace-",
		},
		Status: kapi.NamespaceStatus{},
	}
}

func createServiceProperties() *kapi.Service {
	return &kapi.Service{
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
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
		ObjectMeta: metav1.ObjectMeta{
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
func launchApi(t *testing.T) (routeinternalclientset.Interface, projectinternalclientset.Interface, kclientset.Interface, func(), error) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	kc, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	cfg, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	routeclient, err := routeinternalclientset.NewForConfig(cfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	projectclient, err := projectinternalclientset.NewForConfig(cfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return routeclient, projectclient, kc, func() {
		testserver.CleanupMasterEtcd(t, masterConfig)
	}, nil
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

// launchRateLimitedRouter launches a rate-limited template router
// that communicates with the api via the provided clients.
func launchRateLimitedRouter(t *testing.T, routeclient routeinternalclientset.Interface, projectclient projectinternalclientset.Interface, kc kclientset.Interface, name string, maxDelay int32, reloadInterval time.Duration, reloadedMap map[string]int) *templateplugin.TemplatePlugin {
	reloadedMap[name] = 0
	rateLimitingFunc := func() error {
		reloadedMap[name] += 1
		t.Logf("Router %s reloaded (%d times)\n", name, reloadedMap[name])
		return nil
	}
	var plugin router.Plugin
	templatePlugin, plugin := initializeRouterPlugins(routeclient, projectclient, name, reloadInterval, rateLimitingFunc)

	if maxDelay > 0 {
		plugin = NewDelayPlugin(plugin, maxDelay)
	}

	factory := controllerfactory.NewDefaultRouterControllerFactory(routeclient, projectclient.Project().Projects(), kc)
	ctrl := factory.Create(plugin, false, false)
	ctrl.Run()

	return templatePlugin
}

func initializeRouterPlugins(routeclient routeinternalclientset.Interface, projectclient projectinternalclientset.Interface, name string, reloadInterval time.Duration, rateLimitingFunc ratelimiter.HandlerFunc) (*templateplugin.TemplatePlugin, router.Plugin) {
	r := templateplugin.NewFakeTemplateRouter()

	r.EnableRateLimiter(reloadInterval, func() error {
		r.FakeReloadHandler()
		return rateLimitingFunc()
	})

	tracker := controller.NewSimpleContentionTracker(time.Minute)
	go tracker.Run(wait.NeverStop)
	lease := writerlease.New(time.Minute, 3*time.Second)
	go lease.Run(wait.NeverStop)
	templatePlugin := &templateplugin.TemplatePlugin{Router: r}
	statusPlugin := controller.NewStatusAdmitter(templatePlugin, routeclient.Route(), name, "", lease, tracker)
	validationPlugin := controller.NewExtendedValidator(statusPlugin, controller.RejectionRecorder(statusPlugin))
	uniquePlugin := controller.NewUniqueHost(validationPlugin, controller.HostForRoute, false, controller.RejectionRecorder(statusPlugin))

	return templatePlugin, uniquePlugin
}

// launchRouter launches a template router that communicates with the
// api via the provided clients.
func launchRouter(routeclient routeinternalclientset.Interface, projectclient projectinternalclientset.Interface, kc kclientset.Interface, routerSelection infrarouter.RouterSelection) *templateplugin.TemplatePlugin {
	templatePlugin, plugin := initializeRouterPlugins(routeclient, projectclient, "test-router", 0, func() error {
		return nil
	})
	factory := routerSelection.NewFactory(routeclient, projectclient.Project().Projects(), kc)
	ctrl := factory.Create(plugin, false, false)
	ctrl.Run()

	return templatePlugin
}

func updateNamespaceLabels(t *testing.T, kc kclientset.Interface, namespace *kapi.Namespace, labels map[string]string) {
	err := wait.PollImmediate(waitInterval, wait.ForeverTestTimeout, func() (bool, error) {
		namespace.Labels = labels
		_, err := kc.Core().Namespaces().Update(namespace)
		if errors.IsConflict(err) {
			// The resource was updated by kube machinery.
			// Get the latest version and retry.
			namespace, err = kc.Core().Namespaces().Get(namespace.Name, metav1.GetOptions{})
			return false, err
		}
		return (err == nil), err
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func waitForRouterToHaveRoute(t *testing.T, templatePlugin *templateplugin.TemplatePlugin, route *routeapi.Route, present bool) {
	err := wait.PollImmediate(waitInterval, wait.ForeverTestTimeout, func() (bool, error) {
		if templatePlugin.Router.HasRoute(route) {
			return present, nil
		}
		return !present, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
