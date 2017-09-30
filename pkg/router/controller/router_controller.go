package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	"github.com/openshift/origin/pkg/router"
)

// NamespaceLister returns all the names that should be watched by the client
type NamespaceLister interface {
	NamespaceNames() (sets.String, error)
}

// RouterController abstracts the details of watching resources like Routes, Endpoints, etc.
// used by the plugin implementation.
type RouterController struct {
	lock sync.Mutex

	Plugin router.Plugin

	firstSyncDone       bool
	filteredByNamespace bool

	Namespaces            NamespaceLister
	NamespaceSyncInterval time.Duration
	NamespaceWaitInterval time.Duration
	NamespaceRetries      int

	WatchNodes bool

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
	c.handleFirstSync()
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
			c.Commit()

			return
		}
		utilruntime.HandleError(fmt.Errorf("unable to find namespaces for router: %v", err))
		time.Sleep(c.NamespaceWaitInterval)
	}
	glog.V(4).Infof("Unable to update list of namespaces")
}

// HandleNode handles a single Node event and synchronizes the router backend
func (c *RouterController) HandleNode(eventType watch.EventType, obj interface{}) {
	node := obj.(*kapi.Node)
	c.lock.Lock()
	defer c.lock.Unlock()

	glog.V(4).Infof("Processing Node : %s", node.Name)
	glog.V(4).Infof("           Event: %s", eventType)

	if err := c.Plugin.HandleNode(eventType, node); err != nil {
		utilruntime.HandleError(err)
	}
}

// HandleRoute handles a single Route event and synchronizes the router backend.
func (c *RouterController) HandleRoute(eventType watch.EventType, obj interface{}) {
	route := obj.(*routeapi.Route)
	c.lock.Lock()
	defer c.lock.Unlock()

	c.processRoute(eventType, route)
	c.Commit()
}

// HandleEndpoints handles a single Endpoints event and refreshes the router backend.
func (c *RouterController) HandleEndpoints(eventType watch.EventType, obj interface{}) {
	endpoints := obj.(*kapi.Endpoints)
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.Plugin.HandleEndpoints(eventType, endpoints); err != nil {
		utilruntime.HandleError(err)
	}
	c.Commit()
}

// HandleIngress handles a single Ingress event and synchronizes the router backend.
func (c *RouterController) HandleIngress(eventType watch.EventType, obj interface{}) {
	ingress := obj.(*extensions.Ingress)
	// The ingress translator synchronizes access to its cache with a
	// lock, so calls to it are made outside of the controller lock to
	// avoid unintended interaction.
	events := c.IngressTranslator.TranslateIngressEvent(eventType, ingress)

	c.lock.Lock()
	defer c.lock.Unlock()

	c.processIngressEvents(events)
	c.Commit()
}

// HandleSecret handles a single Secret event and synchronizes the router backend.
func (c *RouterController) HandleSecret(eventType watch.EventType, obj interface{}) {
	secret := obj.(*kapi.Secret)
	// The ingress translator synchronizes access to its cache with a
	// lock, so calls to it are made outside of the controller lock to
	// avoid unintended interaction.
	events := c.IngressTranslator.TranslateSecretEvent(eventType, secret)

	c.lock.Lock()
	defer c.lock.Unlock()

	c.processIngressEvents(events)
	c.Commit()
}

// Commit notifies the plugin that it is safe to commit state.
func (c *RouterController) Commit() {
	if !c.isSyncing() {
		if err := c.Plugin.Commit(); err != nil {
			utilruntime.HandleError(err)
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

func (c *RouterController) handleFirstSync() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.firstSyncDone = true
	glog.V(4).Infof("Router first sync complete")
	c.Commit()
}

func (c *RouterController) isSyncing() bool {
	syncing := false

	if !c.firstSyncDone {
		syncing = true
	} else if c.Namespaces != nil && !c.filteredByNamespace {
		syncing = true
	}
	return syncing
}
