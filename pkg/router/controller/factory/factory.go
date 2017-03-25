package factory

import (
	"fmt"
	"sort"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	kextensionsclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/extensions/internalversion"
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
	"k8s.io/kubernetes/pkg/api/meta"
)

// RouterControllerFactory initializes and manages the watches that drive a router
// controller. It supports optional scoping on Namespace, Labels, and Fields of routes.
// If Namespace is empty, it means "all namespaces".
type RouterControllerFactory struct {
	KClient        kcoreclient.EndpointsGetter
	OSClient       osclient.RoutesNamespacer
	IngressClient  kextensionsclient.IngressesGetter
	SecretClient   kcoreclient.SecretsGetter
	NodeClient     kcoreclient.NodesGetter
	Namespaces     controller.NamespaceLister
	ResyncInterval time.Duration
	Namespace      string
	Labels         labels.Selector
	Fields         fields.Selector
}

// NewDefaultRouterControllerFactory initializes a default router controller factory.
func NewDefaultRouterControllerFactory(oc osclient.RoutesNamespacer, kc kclientset.Interface) *RouterControllerFactory {
	return &RouterControllerFactory{
		KClient:        kc.Core(),
		OSClient:       oc,
		IngressClient:  kc.Extensions(),
		SecretClient:   kc.Core(),
		NodeClient:     kc.Core(),
		ResyncInterval: 10 * time.Minute,

		Namespace: kapi.NamespaceAll,
		Labels:    labels.Everything(),
		Fields:    fields.Everything(),
	}
}

// routerKeyFn comes from MetaNamespaceKeyFunc in vendor/k8s.io/kubernetes/pkg/client/cache/store.go.
// It was modified and added here because there is no way to know if an ExplicitKey was passed before
// adding the UID to prevent an invalid state transistion if deletions and adds happen quickly.
func routerKeyFn(obj interface{}) (string, error) {
	if key, ok := obj.(cache.ExplicitKey); ok {
		return string(key), nil
	}
	meta, err := meta.Accessor(obj)
	if err != nil {
		return "", fmt.Errorf("object has no meta: %v", err)
	}
	if len(meta.GetNamespace()) > 0 {
		return meta.GetNamespace() + "/" + meta.GetName() + "/" + string(meta.GetUID()), nil
	}
	return meta.GetName() + "/" + string(meta.GetUID()), nil
}

// Create begins listing and watching against the API server for the desired route and endpoint
// resources. It spawns child goroutines that cannot be terminated.
func (factory *RouterControllerFactory) Create(plugin router.Plugin, watchNodes, enableIngress bool) *controller.RouterController {
	routeEventQueue := oscache.NewEventQueue(routerKeyFn)
	cache.NewReflector(&routeLW{
		client:    factory.OSClient,
		namespace: factory.Namespace,
		field:     factory.Fields,
		label:     factory.Labels,
	}, &routeapi.Route{}, routeEventQueue, factory.ResyncInterval).Run()

	endpointsEventQueue := oscache.NewEventQueue(routerKeyFn)
	cache.NewReflector(&endpointsLW{
		client:    factory.KClient,
		namespace: factory.Namespace,
		// we do not scope endpoints by labels or fields because the route labels != endpoints labels
	}, &kapi.Endpoints{}, endpointsEventQueue, factory.ResyncInterval).Run()

	nodeEventQueue := oscache.NewEventQueue(routerKeyFn)
	if watchNodes {
		cache.NewReflector(&nodeLW{
			client: factory.NodeClient,
			field:  fields.Everything(),
			label:  labels.Everything(),
		}, &kapi.Node{}, nodeEventQueue, factory.ResyncInterval).Run()
	}

	ingressEventQueue := oscache.NewEventQueue(routerKeyFn)
	secretEventQueue := oscache.NewEventQueue(routerKeyFn)
	var ingressTranslator *controller.IngressTranslator
	if enableIngress {
		ingressTranslator = controller.NewIngressTranslator(factory.SecretClient)

		cache.NewReflector(&ingressLW{
			client:    factory.IngressClient,
			namespace: factory.Namespace,
			// The same filtering is applied to ingress as is applied to routes
			field: factory.Fields,
			label: factory.Labels,
		}, &extensions.Ingress{}, ingressEventQueue, factory.ResyncInterval).Run()

		cache.NewReflector(&secretLW{
			client:    factory.SecretClient,
			namespace: factory.Namespace,
			field:     fields.Everything(),
			label:     labels.Everything(),
		}, &kapi.Secret{}, secretEventQueue, factory.ResyncInterval).Run()
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
		NextIngress: func() (watch.EventType, *extensions.Ingress, error) {
			eventType, obj, err := ingressEventQueue.Pop()
			if err != nil {
				return watch.Error, nil, err
			}
			return eventType, obj.(*extensions.Ingress), nil
		},
		NextSecret: func() (watch.EventType, *kapi.Secret, error) {
			eventType, obj, err := secretEventQueue.Pop()
			if err != nil {
				return watch.Error, nil, err
			}
			return eventType, obj.(*kapi.Secret), nil
		},
		EndpointsListCount: func() int {
			return endpointsEventQueue.ListCount()
		},
		RoutesListCount: func() int {
			return routeEventQueue.ListCount()
		},
		IngressesListCount: func() int {
			return ingressEventQueue.ListCount()
		},
		SecretsListCount: func() int {
			return secretEventQueue.ListCount()
		},
		EndpointsListSuccessfulAtLeastOnce: func() bool {
			return endpointsEventQueue.ListSuccessfulAtLeastOnce()
		},
		RoutesListSuccessfulAtLeastOnce: func() bool {
			return routeEventQueue.ListSuccessfulAtLeastOnce()
		},
		IngressesListSuccessfulAtLeastOnce: func() bool {
			return ingressEventQueue.ListSuccessfulAtLeastOnce()
		},
		SecretsListSuccessfulAtLeastOnce: func() bool {
			return secretEventQueue.ListSuccessfulAtLeastOnce()
		},
		EndpointsListConsumed: func() bool {
			return endpointsEventQueue.ListConsumed()
		},
		RoutesListConsumed: func() bool {
			return routeEventQueue.ListConsumed()
		},
		IngressesListConsumed: func() bool {
			return ingressEventQueue.ListConsumed()
		},
		SecretsListConsumed: func() bool {
			return secretEventQueue.ListConsumed()
		},
		Namespaces: factory.Namespaces,
		// check namespaces a bit more often than we resync events, so that we aren't always waiting
		// the maximum interval for new items to come into the list
		// TODO: trigger a reflector resync after every namespace sync?
		NamespaceSyncInterval: factory.ResyncInterval - 10*time.Second,
		NamespaceWaitInterval: 10 * time.Second,
		NamespaceRetries:      5,
		WatchNodes:            watchNodes,
		EnableIngress:         enableIngress,
		IngressTranslator:     ingressTranslator,
	}
}

// CreateNotifier begins listing and watching against the API server for the desired route and endpoint
// resources. It spawns child goroutines that cannot be terminated. It is a more efficient store of a
// route system.
func (factory *RouterControllerFactory) CreateNotifier(changed func()) RoutesByHost {
	keyFn := routerKeyFn
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
	client    kcoreclient.EndpointsGetter
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
	client kcoreclient.NodesGetter
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

// ingressAge sorts ingress resources from oldest to newest and is stable for all of them.
type ingressAge []extensions.Ingress

func (ia ingressAge) Len() int      { return len(ia) }
func (ia ingressAge) Swap(i, j int) { ia[i], ia[j] = ia[j], ia[i] }
func (ia ingressAge) Less(i, j int) bool {
	ingress1 := ia[i]
	ingress2 := ia[j]
	if ingress1.CreationTimestamp.Before(ingress2.CreationTimestamp) {
		return true
	}
	if ingress2.CreationTimestamp.Before(ingress1.CreationTimestamp) {
		return false
	}
	return ingress1.UID < ingress2.UID
}

// ingressLW is a ListWatcher for ingress that can be filtered to a label, field, or
// namespace.
type ingressLW struct {
	client    kextensionsclient.IngressesGetter
	label     labels.Selector
	field     fields.Selector
	namespace string
}

func (lw *ingressLW) List(options kapi.ListOptions) (runtime.Object, error) {
	opts := kapi.ListOptions{
		LabelSelector: lw.label,
		FieldSelector: lw.field,
	}
	ingresses, err := lw.client.Ingresses(lw.namespace).List(opts)
	if err != nil {
		return nil, err
	}
	// return ingress in order of age to avoid rejections during resync
	sort.Sort(ingressAge(ingresses.Items))
	return ingresses, nil
}

func (lw *ingressLW) Watch(options kapi.ListOptions) (watch.Interface, error) {
	opts := kapi.ListOptions{
		LabelSelector:   lw.label,
		FieldSelector:   lw.field,
		ResourceVersion: options.ResourceVersion,
	}
	return lw.client.Ingresses(lw.namespace).Watch(opts)
}

// secretLW is a list watcher for routes.
type secretLW struct {
	client    kcoreclient.SecretsGetter
	label     labels.Selector
	field     fields.Selector
	namespace string
}

func (lw *secretLW) List(options kapi.ListOptions) (runtime.Object, error) {
	return lw.client.Secrets(lw.namespace).List(options)
}

func (lw *secretLW) Watch(options kapi.ListOptions) (watch.Interface, error) {
	opts := kapi.ListOptions{
		LabelSelector:   lw.label,
		FieldSelector:   lw.field,
		ResourceVersion: options.ResourceVersion,
	}
	return lw.client.Secrets(lw.namespace).Watch(opts)
}
