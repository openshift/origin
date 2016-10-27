package factory

import (
	"fmt"
	"sort"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
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
	NodeClient     kclient.NodesInterface
	Namespaces     controller.NamespaceLister
	ResyncInterval time.Duration
	Namespace      string
	Labels         labels.Selector
	Fields         fields.Selector
}

// NewDefaultRouterControllerFactory initializes a default router controller factory.
func NewDefaultRouterControllerFactory(oc osclient.RoutesNamespacer, kc kclient.Interface) *RouterControllerFactory {
	return &RouterControllerFactory{
		KClient:        kc,
		OSClient:       oc,
		NodeClient:     kc,
		ResyncInterval: 10 * time.Minute,

		Namespace: kapi.NamespaceAll,
		Labels:    labels.Everything(),
		Fields:    fields.Everything(),
	}
}

// Create begins listing and watching against the API server for the desired route and endpoint
// resources. It spawns child goroutines that cannot be terminated.
func (factory *RouterControllerFactory) Create(plugin router.Plugin, watchNodes bool) *controller.RouterController {
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

	nodeEventQueue := oscache.NewEventQueue(cache.MetaNamespaceKeyFunc)
	if watchNodes {
		cache.NewReflector(&nodeLW{
			client: factory.NodeClient,
			field:  fields.Everything(),
			label:  labels.Everything(),
		}, &kapi.Node{}, nodeEventQueue, factory.ResyncInterval).Run()
	}

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
		NextNode: func() (watch.EventType, *kapi.Node, error) {
			eventType, obj, err := nodeEventQueue.Pop()
			if err != nil {
				return watch.Error, nil, err
			}
			return eventType, obj.(*kapi.Node), nil
		},
		EndpointsListConsumed: func() bool {
			return endpointsEventQueue.ListConsumed()
		},
		RoutesListConsumed: func() bool {
			return routeEventQueue.ListConsumed()
		},
		Namespaces: factory.Namespaces,
		// check namespaces a bit more often than we resync events, so that we aren't always waiting
		// the maximum interval for new items to come into the list
		// TODO: trigger a reflector resync after every namespace sync?
		NamespaceSyncInterval: factory.ResyncInterval - 10*time.Second,
		NamespaceWaitInterval: 10 * time.Second,
		NamespaceRetries:      5,
		WatchNodes:            watchNodes,
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

	go utilwait.Until(func() {
		for {
			if _, _, err := routeEventQueue.Pop(); err != nil {
				return
			}
			changed()
		}
	}, time.Second, utilwait.NeverStop)
	go utilwait.Until(func() {
		for {
			if _, _, err := endpointsEventQueue.Pop(); err != nil {
				return
			}
			changed()
		}
	}, time.Second, utilwait.NeverStop)

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

// routeAge sorts routes from oldest to newest and is stable for all routes.
type routeAge []routeapi.Route

func (r routeAge) Len() int      { return len(r) }
func (r routeAge) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r routeAge) Less(i, j int) bool {
	return routeapi.RouteLessThan(&r[i], &r[j])
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
	if len(route.Spec.Host) > 0 {
		hosts = append(hosts, route.Spec.Host)
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

func (lw *routeLW) List(options kapi.ListOptions) (runtime.Object, error) {
	opts := kapi.ListOptions{
		LabelSelector: lw.label,
		FieldSelector: lw.field,
	}
	routes, err := lw.client.Routes(lw.namespace).List(opts)
	if err != nil {
		return nil, err
	}
	// return routes in order of age to avoid rejections during resync
	sort.Sort(routeAge(routes.Items))
	return routes, nil
}

func (lw *routeLW) Watch(options kapi.ListOptions) (watch.Interface, error) {
	opts := kapi.ListOptions{
		LabelSelector:   lw.label,
		FieldSelector:   lw.field,
		ResourceVersion: options.ResourceVersion,
	}
	return lw.client.Routes(lw.namespace).Watch(opts)
}

// endpointsLW is a list watcher for routes.
type endpointsLW struct {
	client    kclient.EndpointsNamespacer
	label     labels.Selector
	field     fields.Selector
	namespace string
}

func (lw *endpointsLW) List(options kapi.ListOptions) (runtime.Object, error) {
	return lw.client.Endpoints(lw.namespace).List(options)
}

func (lw *endpointsLW) Watch(options kapi.ListOptions) (watch.Interface, error) {
	opts := kapi.ListOptions{
		LabelSelector:   lw.label,
		FieldSelector:   lw.field,
		ResourceVersion: options.ResourceVersion,
	}
	return lw.client.Endpoints(lw.namespace).Watch(opts)
}

// nodeLW is a list watcher for nodes.
type nodeLW struct {
	client kclient.NodesInterface
	label  labels.Selector
	field  fields.Selector
}

func (lw *nodeLW) List(options kapi.ListOptions) (runtime.Object, error) {
	return lw.client.Nodes().List(options)
}

func (lw *nodeLW) Watch(options kapi.ListOptions) (watch.Interface, error) {
	opts := kapi.ListOptions{
		LabelSelector:   lw.label,
		FieldSelector:   lw.field,
		ResourceVersion: options.ResourceVersion,
	}
	return lw.client.Nodes().Watch(opts)
}
