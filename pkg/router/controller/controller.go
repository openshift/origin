package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
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
	NextIngress   func() (watch.EventType, *extensions.Ingress, error)
	NextSecret    func() (watch.EventType, *kapi.Secret, error)

	RoutesListConsumed    func() bool
	EndpointsListConsumed func() bool
	IngressesListConsumed func() bool
	SecretsListConsumed   func() bool
	routesListConsumed    bool
	endpointsListConsumed bool
	ingressesListConsumed bool
	secretsListConsumed   bool
	filteredByNamespace   bool
	syncing               bool

	RoutesListSuccessfulAtLeastOnce    func() bool
	EndpointsListSuccessfulAtLeastOnce func() bool
	IngressesListSuccessfulAtLeastOnce func() bool
	SecretsListSuccessfulAtLeastOnce   func() bool
	RoutesListCount                    func() int
	EndpointsListCount                 func() int
	IngressesListCount                 func() int
	SecretsListCount                   func() int

	WatchNodes bool

	Namespaces            NamespaceLister
	NamespaceSyncInterval time.Duration
	NamespaceWaitInterval time.Duration
	NamespaceRetries      int

	EnableIngress     bool
	IngressTranslator *IngressTranslator
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
	if c.EnableIngress {
		go utilwait.Forever(c.HandleIngress, 0)
		go utilwait.Forever(c.HandleSecret, 0)
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
		(c.Namespaces == nil || c.filteredByNamespace) &&
		(!c.EnableIngress ||
			(c.IngressesListSuccessfulAtLeastOnce() && c.SecretsListSuccessfulAtLeastOnce()))
	if !synced {
		return false
	}

	// If any of the event queues were empty after the initial List,
	// the tracking listConsumed variable's default value of 'false'
	// may prevent the router from committing.  Set the value to
	// 'true' to ensure that state can be committed if necessary.
	if c.RoutesListCount() == 0 {
		c.routesListConsumed = true
	}
	if c.EndpointsListCount() == 0 {
		c.endpointsListConsumed = true
	}
	if c.EnableIngress {
		if c.IngressesListCount() == 0 {
			c.ingressesListConsumed = true
		}
		if c.SecretsListCount() == 0 {
			c.secretsListConsumed = true
		}
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

			// The ingress translator synchronizes access to its cache with a
			// lock, so calls to it are made outside of the controller lock to
			// avoid unintended interaction.
			if c.EnableIngress {
				c.IngressTranslator.UpdateNamespaces(namespaces)
			}

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

	c.processRoute(eventType, route)

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

// HandleIngress handles a single Ingress event and synchronizes the router backend.
func (c *RouterController) HandleIngress() {
	eventType, ingress, err := c.NextIngress()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to read ingress: %v", err))
		return
	}

	// The ingress translator synchronizes access to its cache with a
	// lock, so calls to it are made outside of the controller lock to
	// avoid unintended interaction.
	events := c.IngressTranslator.TranslateIngressEvent(eventType, ingress)

	c.lock.Lock()
	defer c.lock.Unlock()

	c.processIngressEvents(events)

	// Change the local sync state within the lock to ensure that all
	// event handlers have the same view of sync state.
	c.ingressesListConsumed = c.IngressesListConsumed()
	c.commit()
}

// HandleSecret handles a single Secret event and synchronizes the router backend.
func (c *RouterController) HandleSecret() {
	eventType, secret, err := c.NextSecret()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to read secret: %v", err))
		return

	}

	// The ingress translator synchronizes access to its cache with a
	// lock, so calls to it are made outside of the controller lock to
	// avoid unintended interaction.
	events := c.IngressTranslator.TranslateSecretEvent(eventType, secret)

	c.lock.Lock()
	defer c.lock.Unlock()

	c.processIngressEvents(events)

	// Change the local sync state within the lock to ensure that all
	// event handlers have the same view of sync state.
	c.secretsListConsumed = c.SecretsListConsumed()
	c.commit()
}

// commit notifies the plugin that it is safe to commit state.
func (c *RouterController) commit() {
	syncing := !(c.endpointsListConsumed && c.routesListConsumed &&
		(c.Namespaces == nil || c.filteredByNamespace) &&
		(!c.EnableIngress ||
			(c.ingressesListConsumed && c.secretsListConsumed)))
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

// processRoute logs and propagates a route event to the plugin
func (c *RouterController) processRoute(eventType watch.EventType, route *routeapi.Route) {
	glog.V(4).Infof("Processing Route: %s/%s -> %s", route.Namespace, route.Name, route.Spec.To.Name)
	glog.V(4).Infof("           Alias: %s", route.Spec.Host)
	glog.V(4).Infof("           Path: %s", route.Spec.Path)
	glog.V(4).Infof("           Event: %s", eventType)

	if err := c.Plugin.HandleRoute(eventType, route); err != nil {
		utilruntime.HandleError(err)
	}
}

// processIngressEvents logs and propagates the route events resulting from an ingress or secret event
func (c *RouterController) processIngressEvents(events []ingressRouteEvents) {
	for _, ingressEvent := range events {
		glog.V(4).Infof("Processing Ingress %s", ingressEvent.ingressKey)
		for _, routeEvent := range ingressEvent.routeEvents {
			c.processRoute(routeEvent.eventType, routeEvent.route)
		}
	}
}
