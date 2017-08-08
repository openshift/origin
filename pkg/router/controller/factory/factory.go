package factory

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	routesinformerfactory "github.com/openshift/origin/pkg/route/generated/informers/internalversion"
	routeinternalclientset "github.com/openshift/origin/pkg/route/generated/internalclientset"
	"github.com/openshift/origin/pkg/router"
	routercontroller "github.com/openshift/origin/pkg/router/controller"
	informerfactory "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
)

// RouterControllerFactory initializes and manages the watches that drive a router
// controller. It supports optional scoping on Namespace, Labels, and Fields of routes.
// If Namespace is empty, it means "all namespaces".
type RouterControllerFactory struct {
	SecretClient kcoreclient.SecretsGetter

	Namespaces     routercontroller.NamespaceLister
	ResyncInterval time.Duration
	Namespace      string
	Labels         labels.Selector
	Fields         fields.Selector
	IFactory       informerfactory.SharedInformerFactory
	RFactory       routesinformerfactory.SharedInformerFactory
}

// NewDefaultRouterControllerFactory initializes a default router controller factory.
func NewDefaultRouterControllerFactory(oc routeinternalclientset.Interface, kc kclientset.Interface) *RouterControllerFactory {
	resyncInterval := 10 * time.Minute
	return &RouterControllerFactory{
		SecretClient:   kc.Core(),
		ResyncInterval: resyncInterval,

		Namespace: metav1.NamespaceAll,
		Labels:    labels.Everything(),
		Fields:    fields.Everything(),
		IFactory:  informerfactory.NewSharedInformerFactory(kc, resyncInterval),
		RFactory:  routesinformerfactory.NewSharedInformerFactory(oc, resyncInterval),
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
func (factory *RouterControllerFactory) Create(plugin router.Plugin, watchNodes, enableIngress bool) *routercontroller.RouterController {

	endpointsInformer := factory.IFactory.Core().InternalVersion().Endpoints()
	routesInformer := factory.RFactory.Route().InternalVersion().Routes()
	nodesInformer := factory.IFactory.Core().InternalVersion().Nodes()
	ingressInformer := factory.IFactory.Extensions().InternalVersion().Ingresses()
	secretsInformer := factory.IFactory.Core().InternalVersion().Secrets()

	return &routercontroller.RouterController{
		Plugin: plugin,
		StartRouteWatch: func(handler cache.ResourceEventHandler) {
			routesInformer.Informer().AddEventHandler(handler)
			go routesInformer.Informer().Run(utilwait.NeverStop)
		},
		StartEndpointsWatch: func(handler cache.ResourceEventHandler) {
			endpointsInformer.Informer().AddEventHandler(handler)
			go endpointsInformer.Informer().Run(utilwait.NeverStop)
		},
		StartNodeWatch: func(handler cache.ResourceEventHandler) {
			nodesInformer.Informer().AddEventHandler(handler)
			go nodesInformer.Informer().Run(utilwait.NeverStop)
		},
		StartIngressWatch: func(handler cache.ResourceEventHandler) {
			ingressInformer.Informer().AddEventHandler(handler)
			go ingressInformer.Informer().Run(utilwait.NeverStop)
		},
		StartSecretsWatch: func(handler cache.ResourceEventHandler) {
			secretsInformer.Informer().AddEventHandler(handler)
			go secretsInformer.Informer().Run(utilwait.NeverStop)
		},
		HasSynced: func() bool {
			var stopCh chan struct{}
			cache.WaitForCacheSync(stopCh, routesInformer.Informer().HasSynced,
				endpointsInformer.Informer().HasSynced)
			if enableIngress {
				cache.WaitForCacheSync(stopCh, secretsInformer.Informer().HasSynced,
					ingressInformer.Informer().HasSynced)
			}
			if watchNodes {
				cache.WaitForCacheSync(stopCh, nodesInformer.Informer().HasSynced)
			}
			return true
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
		IngressTranslator:     routercontroller.NewIngressTranslator(factory.SecretClient),
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
