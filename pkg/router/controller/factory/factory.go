package factory

import (
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/golang/glog"

	idling "github.com/openshift/service-idler/pkg/apis/idling/v1alpha2"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	kcache "k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/router"
	routercontroller "github.com/openshift/origin/pkg/router/controller"
)

const (
	DefaultResyncInterval = 30 * time.Minute
)

// RouterControllerFactory initializes and manages the watches that drive a router
// controller. It supports optional scoping on Namespace, Labels, and Fields of routes.
// If Namespace is empty, it means "all namespaces".
type RouterControllerFactory struct {
	*RouterInformerFactory
}

// NewRouterControllerFactory initializes a router controller factory.
func NewRouterControllerFactory(informerFactory *RouterInformerFactory) *RouterControllerFactory {
	return &RouterControllerFactory{
		RouterInformerFactory: informerFactory,
	}
}

// Create begins listing and watching against the API server for the desired route and endpoint
// resources. It spawns child goroutines that cannot be terminated.
func (f *RouterControllerFactory) Create(plugin router.Plugin, watchNodes, enableUnidling bool) *routercontroller.RouterController {
	rc := &routercontroller.RouterController{
		Plugin:         plugin,
		WatchNodes:     watchNodes,
		EnableUnidling: enableUnidling,

		NamespaceLabels:        f.NamespaceLabels,
		FilteredNamespaceNames: make(sets.String),
		NamespaceRoutes:        make(map[string]map[string]*routeapi.Route),
		NamespaceEndpoints:     make(map[string]map[string]*kapi.Endpoints),

		ProjectClient:       f.ProjectClient,
		ProjectLabels:       f.ProjectLabels,
		ProjectWaitInterval: 10 * time.Second,
		ProjectRetries:      5,
	}

	// Check projects a bit more often than we resync events, so that we aren't always waiting
	// the maximum interval for new items to come into the list
	if f.ResyncInterval > 10*time.Second {
		rc.ProjectSyncInterval = f.ResyncInterval - 10*time.Second
	} else {
		rc.ProjectSyncInterval = f.ResyncInterval
	}

	f.initInformers(rc)
	f.processExistingItems(rc)
	f.registerInformerEventHandlers(rc)
	return rc
}

func (f *RouterControllerFactory) initInformers(rc *routercontroller.RouterController) {
	if f.NamespaceLabels != nil {
		f.InformerFor(&kapi.Namespace{})
	}
	f.InformerFor(&routeapi.Route{})
	f.InformerFor(&kapi.Endpoints{})
	f.InformerFor(&routeapi.Route{})

	if rc.WatchNodes {
		f.InformerFor(&kapi.Node{})
	}
	if rc.EnableUnidling {
		f.InformerFor(&idling.Idler{})
	}

	f.RouterInformerFactory.Run()
}

func (f *RouterControllerFactory) registerInformerEventHandlers(rc *routercontroller.RouterController) {
	if f.NamespaceLabels != nil {
		f.registerSharedInformerEventHandlers(&kapi.Namespace{}, rc.HandleNamespace)
	}
	f.registerSharedInformerEventHandlers(&kapi.Endpoints{}, rc.HandleEndpoints)

	f.registerSharedInformerEventHandlers(&routeapi.Route{}, rc.HandleRoute)

	if rc.WatchNodes {
		f.registerSharedInformerEventHandlers(&kapi.Node{}, rc.HandleNode)
	}
	if rc.EnableUnidling {
		f.registerSharedInformerEventHandlers(&idling.Idler{}, rc.HandleIdler)
	}
}

func (f *RouterControllerFactory) informerStoreList(obj runtime.Object) []interface{} {
	objType := reflect.TypeOf(obj)
	informer, ok := f.informers[objType]
	if !ok {
		utilruntime.HandleError(fmt.Errorf("listing items failed: %+v shared informer not found", objType))
		return []interface{}{obj}
	}
	return informer.GetStore().List()
}

// processExistingItems processes all existing resource items before doing the first router sync.
// We do not want to persist partial router state for the first time to avoid 503 http errors.
// Relying on informer watch resource will not tell whether all the existing items are consumed.
// So to overcome this, we do:
// - Launch all informers with no registered event handlers
// - Wait for all informers to sync the cache
// - Block router sync
// - Fetch existing items from informers cache and process manually
// - Perform first router sync
// - Register informer event handlers for new updates and resyncs
func (f *RouterControllerFactory) processExistingItems(rc *routercontroller.RouterController) {
	if f.NamespaceLabels != nil {
		items := f.informerStoreList(&kapi.Namespace{})
		if len(items) == 0 {
			rc.UpdateNamespaces()
		} else {
			for _, item := range items {
				rc.HandleNamespace(watch.Added, item.(*kapi.Namespace))
			}
		}
	}

	for _, item := range f.informerStoreList(&kapi.Endpoints{}) {
		rc.HandleEndpoints(watch.Added, item.(*kapi.Endpoints))
	}

	items := []routeapi.Route{}
	for _, item := range f.informerStoreList(&routeapi.Route{}) {
		items = append(items, *(item.(*routeapi.Route)))
	}
	// Return routes in order of age to avoid rejections during resync
	sort.Sort(routeAge(items))
	for i := range items {
		rc.HandleRoute(watch.Added, &items[i])
	}

	if rc.WatchNodes {
		for _, item := range f.informerStoreList(&kapi.Node{}) {
			rc.HandleNode(watch.Added, item.(*kapi.Node))
		}
	}
}

func (f *RouterControllerFactory) registerSharedInformerEventHandlers(obj runtime.Object,
	handleFunc func(watch.EventType, interface{})) {
	informer, ok := f.StrictInformerFor(obj)
	objType := reflect.TypeOf(obj)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("register event handler failed: %+v shared informer not found", objType))
		return
	}

	informer.AddEventHandler(kcache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			handleFunc(watch.Added, obj)
		},
		UpdateFunc: func(_, obj interface{}) {
			handleFunc(watch.Modified, obj)
		},
		DeleteFunc: func(obj interface{}) {
			if objType != reflect.TypeOf(obj) {
				tombstone, ok := obj.(kcache.DeletedFinalStateUnknown)
				if !ok {
					glog.Errorf("Couldn't get object from tombstone: %+v", obj)
					return
				}

				obj = tombstone.Obj
				if objType != reflect.TypeOf(obj) {
					glog.Errorf("Tombstone contained object that is not a %s: %+v", objType, obj)
					return
				}
			}
			handleFunc(watch.Deleted, obj)
		},
	})
}

// routeAge sorts routes from oldest to newest and is stable for all routes.
type routeAge []routeapi.Route

func (r routeAge) Len() int      { return len(r) }
func (r routeAge) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r routeAge) Less(i, j int) bool {
	return routeapi.RouteLessThan(&r[i], &r[j])
}
