package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
)

// NamespaceLister returns all the names that should be watched by the client
type NamespaceLister interface {
	NamespaceNames() (sets.String, error)
}

// RouterController abstracts the details of watching the Route and Endpoints
// resources from the Plugin implementation being used.
type RouterController struct {
	lock sync.Mutex

	Plugin        router.Plugin
	NextRoute     func() (watch.EventType, *routeapi.Route, error)
	NextNode      func() (watch.EventType, *kapi.Node, error)
	NextEndpoints func() (watch.EventType, *kapi.Endpoints, error)

	RoutesListConsumed    func() bool
	EndpointsListConsumed func() bool
	routesListConsumed    bool
	endpointsListConsumed bool
	filteredByNamespace   bool
	syncing               bool

	RoutesListSuccessfulAtLeastOnce    func() bool
	EndpointsListSuccessfulAtLeastOnce func() bool
	RoutesListCount                    func() int
	EndpointsListCount                 func() int

	WatchNodes bool

	Namespaces            NamespaceLister
	NamespaceSyncInterval time.Duration
	NamespaceWaitInterval time.Duration
	NamespaceRetries      int
}

// Run begins watching and syncing.
func (c *RouterController) Run() {
	glog.V(4).Info("Running router controller")
	if c.Namespaces != nil {
		c.HandleNamespaces()
		go utilwait.Forever(c.HandleNamespaces, c.NamespaceSyncInterval)
	}
	go utilwait.Forever(c.HandleRoute, 0)
	go utilwait.Forever(c.HandleEndpoints, 0)
	if c.WatchNodes {
		go utilwait.Forever(c.HandleNode, 0)
	}
	go c.watchForFirstSync()
}

// handleFirstSync signals the router when it sees that the various
// watchers have successfully listed data from the api.
func (c *RouterController) handleFirstSync() bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	synced := c.RoutesListSuccessfulAtLeastOnce() &&
		c.EndpointsListSuccessfulAtLeastOnce() &&
		(c.Namespaces == nil || c.filteredByNamespace)
	if !synced {
		return false
	}

	// If either of the event queues were empty after the initial
	// List, the tracking listConsumed variable's default value of
	// 'false' may prevent the router from committing the readiness
	// status.  Set the value to 'true' to ensure that state will be
	// committed if necessary.
	if c.RoutesListCount() == 0 {
		c.routesListConsumed = true
	}
	if c.EndpointsListCount() == 0 {
		c.endpointsListConsumed = true
	}
	c.commit()

	return true
}

// watchForFirstSync loops until the first sync has been handled.
func (c *RouterController) watchForFirstSync() {
	for {
		if c.handleFirstSync() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (c *RouterController) HandleNamespaces() {
	for i := 0; i < c.NamespaceRetries; i++ {
		namespaces, err := c.Namespaces.NamespaceNames()
		if err == nil {
			c.lock.Lock()
			defer c.lock.Unlock()

			glog.V(4).Infof("Updating watched namespaces: %v", namespaces)
			if err := c.Plugin.HandleNamespaces(namespaces); err != nil {
				utilruntime.HandleError(err)
			}

			// Namespace filtering is assumed to be have been
			// performed so long as the plugin event handler is called
			// at least once.
			c.filteredByNamespace = true
			c.commit()

			return
		}
		utilruntime.HandleError(fmt.Errorf("unable to find namespaces for router: %v", err))
		time.Sleep(c.NamespaceWaitInterval)
	}
	glog.V(4).Infof("Unable to update list of namespaces")
}

// HandleNode handles a single Node event and synchronizes the router backend
func (c *RouterController) HandleNode() {
	eventType, node, err := c.NextNode()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to read nodes: %v", err))
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	glog.V(4).Infof("Processing Node : %s", node.Name)
	glog.V(4).Infof("           Event: %s", eventType)

	if err := c.Plugin.HandleNode(eventType, node); err != nil {
		utilruntime.HandleError(err)
	}
}

// HandleRoute handles a single Route event and synchronizes the router backend.
func (c *RouterController) HandleRoute() {
	eventType, route, err := c.NextRoute()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to read routes: %v", err))
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	glog.V(4).Infof("Processing Route: %s -> %s", route.Name, route.Spec.To.Name)
	glog.V(4).Infof("           Alias: %s", route.Spec.Host)
	glog.V(4).Infof("           Event: %s", eventType)

	if err := c.Plugin.HandleRoute(eventType, route); err != nil {
		utilruntime.HandleError(err)
	}

	// Change the local sync state within the lock to ensure that all
	// event handlers have the same view of sync state.
	c.routesListConsumed = c.RoutesListConsumed()
	c.commit()
}

// HandleEndpoints handles a single Endpoints event and refreshes the router backend.
func (c *RouterController) HandleEndpoints() {
	eventType, endpoints, err := c.NextEndpoints()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to read endpoints: %v", err))
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.Plugin.HandleEndpoints(eventType, endpoints); err != nil {
		utilruntime.HandleError(err)
	}

	// Change the local sync state within the lock to ensure that all
	// event handlers have the same view of sync state.
	c.endpointsListConsumed = c.EndpointsListConsumed()
	c.commit()
}

// commit notifies the plugin that it is safe to commit state.
func (c *RouterController) commit() {
	syncing := !(c.endpointsListConsumed && c.routesListConsumed &&
		(c.Namespaces == nil || c.filteredByNamespace))
	c.logSyncState(syncing)
	if syncing {
		return
	}
	if err := c.Plugin.Commit(); err != nil {
		utilruntime.HandleError(err)
	}
}

func (c *RouterController) logSyncState(syncing bool) {
	if c.syncing != syncing {
		c.syncing = syncing
		if c.syncing {
			glog.V(4).Infof("Router sync in progress")
		} else {
			glog.V(4).Infof("Router sync complete")
		}
	}
}
