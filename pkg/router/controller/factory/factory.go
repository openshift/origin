package factory

import (
	"fmt"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
	"github.com/openshift/origin/pkg/router/controller"
)

// RouterControllerFactory initializes and manages the watches that drive a router
// controller. It supports optional scoping on Namespace, Labels, and Fields of routes.
// If Namespace is empty, it means "all namespaces".
type RouterControllerFactory struct {
	KClient        kclient.EndpointsNamespacer
	OSClient       osclient.RoutesNamespacer
	ResyncInterval time.Duration
	Namespace      string
	Labels         labels.Selector
	Fields         fields.Selector
}

// NewDefaultRouterControllerFactory initializes a default router controller factory.
func NewDefaultRouterControllerFactory(oc osclient.RoutesNamespacer, kc kclient.EndpointsNamespacer) *RouterControllerFactory {
	return &RouterControllerFactory{
		KClient:        kc,
		OSClient:       oc,
		ResyncInterval: 10 * time.Minute,

		Namespace: kapi.NamespaceAll,
		Labels:    labels.Everything(),
		Fields:    fields.Everything(),
	}
}

// Create begins listing and watching against the API server for the desired route and endpoint
// resources. It spawns child goroutines that cannot be terminated.
func (factory *RouterControllerFactory) Create(plugin router.Plugin) *controller.RouterController {
	routeEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&routeLW{
		client:    factory.OSClient,
		namespace: factory.Namespace,
		field:     factory.Fields,
		label:     factory.Labels,
	}, &routeapi.Route{}, routeEventQueue, factory.ResyncInterval).Run()

	endpointsEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&endpointsLW{
		client:    factory.KClient,
		namespace: factory.Namespace,
		// we do not scope endpoints by labels or fields because the route labels != endpoints labels
	}, &kapi.Endpoints{}, endpointsEventQueue, factory.ResyncInterval).Run()

	return &controller.RouterController{
		Plugin: plugin,
		NextEndpoints: func() (watch.EventType, *kapi.Endpoints, error) {
			eventType, obj, err := endpointsEventQueue.Pop()
			if err != nil {
				return watch.Error, nil, err
			}
			return eventType, obj.(*kapi.Endpoints), nil
		},
		NextRoute: func() (watch.EventType, *routeapi.Route, error) {
			eventType, obj, err := routeEventQueue.Pop()
			if err != nil {
				return watch.Error, nil, err
			}
			return eventType, obj.(*routeapi.Route), nil
		},
	}
}

// CreateNotifier begins listing and watching against the API server for the desired route and endpoint
// resources. It spawns child goroutines that cannot be terminated. It is a more efficient store of a
// route system.
func (factory *RouterControllerFactory) CreateNotifier(changed func()) RoutesByHost {
	keyFn := cache.MetaNamespaceKeyFunc
	routeStore := cache.NewIndexer(keyFn, cache.Indexers{"host": hostIndexFunc})
	routeEventQueue := oscache.NewEventQueueForStore(keyFn, routeStore)
	cache.NewReflector(&routeLW{
		client:    factory.OSClient,
		namespace: factory.Namespace,
		field:     factory.Fields,
		label:     factory.Labels,
	}, &routeapi.Route{}, routeEventQueue, factory.ResyncInterval).Run()

	endpointStore := cache.NewStore(keyFn)
	endpointsEventQueue := oscache.NewEventQueueForStore(keyFn, endpointStore)
	cache.NewReflector(&endpointsLW{
		client:    factory.KClient,
		namespace: factory.Namespace,
		// we do not scope endpoints by labels or fields because the route labels != endpoints labels
	}, &kapi.Endpoints{}, endpointsEventQueue, factory.ResyncInterval).Run()

	go util.Until(func() {
		for {
			if _, _, err := routeEventQueue.Pop(); err != nil {
				return
			}
			changed()
		}
	}, time.Second, util.NeverStop)
	go util.Until(func() {
		for {
			if _, _, err := endpointsEventQueue.Pop(); err != nil {
				return
			}
			changed()
		}
	}, time.Second, util.NeverStop)

	return &routesByHost{
		routes:    routeStore,
		endpoints: endpointStore,
	}
}

type RoutesByHost interface {
	Hosts() []string
	Route(host string) (*routeapi.Route, bool)
	Endpoints(namespace, name string) *kapi.Endpoints
}

type routesByHost struct {
	routes    cache.Indexer
	endpoints cache.Store
}

func (r *routesByHost) Hosts() []string {
	return r.routes.ListIndexFuncValues("host")
}

func (r *routesByHost) Route(host string) (*routeapi.Route, bool) {
	arr, err := r.routes.ByIndex("host", host)
	if err != nil || len(arr) == 0 {
		return nil, false
	}
	return oldestRoute(arr), true
}

func (r *routesByHost) Endpoints(namespace, name string) *kapi.Endpoints {
	obj, ok, err := r.endpoints.GetByKey(fmt.Sprintf("%s/%s", namespace, name))
	if !ok || err != nil {
		return &kapi.Endpoints{}
	}
	return obj.(*kapi.Endpoints)
}

func oldestRoute(routes []interface{}) *routeapi.Route {
	var oldest *routeapi.Route
	for i := range routes {
		route := routes[i].(*routeapi.Route)
		if oldest == nil || route.CreationTimestamp.Before(oldest.CreationTimestamp) {
			oldest = route
		}
	}
	return oldest
}

func hostIndexFunc(obj interface{}) ([]string, error) {
	route := obj.(*routeapi.Route)
	hosts := []string{
		fmt.Sprintf("%s-%s%s", route.Name, route.Namespace, ".generated.local"),
	}
	if len(route.Host) > 0 {
		hosts = append(hosts, route.Host)
	}
	return hosts, nil
}

// routeLW is a ListWatcher for routes that can be filtered to a label, field, or
// namespace.
type routeLW struct {
	client    osclient.RoutesNamespacer
	label     labels.Selector
	field     fields.Selector
	namespace string
}

func (lw *routeLW) List() (runtime.Object, error) {
	return lw.client.Routes(lw.namespace).List(lw.label, lw.field)
}

func (lw *routeLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.Routes(lw.namespace).Watch(lw.label, lw.field, resourceVersion)
}

// endpointsLW is a list watcher for routes.
type endpointsLW struct {
	client    kclient.EndpointsNamespacer
	label     labels.Selector
	field     fields.Selector
	namespace string
}

func (lw *endpointsLW) List() (runtime.Object, error) {
	return lw.client.Endpoints(lw.namespace).List(labels.Everything())
}

func (lw *endpointsLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.Endpoints(lw.namespace).Watch(lw.label, lw.field, resourceVersion)
}
