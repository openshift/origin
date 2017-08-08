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
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
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

	Plugin router.Plugin

	// Functions for the Informer implementations
	StartRouteWatch     func(cache.ResourceEventHandler)
	StartEndpointsWatch func(cache.ResourceEventHandler)
	StartNodeWatch      func(cache.ResourceEventHandler)
	StartIngressWatch   func(cache.ResourceEventHandler)
	StartSecretsWatch   func(cache.ResourceEventHandler)
	HasSynced           func() bool

	filteredByNamespace bool
	syncing             bool

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

	c.StartRouteWatch(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			route := obj.(*routeapi.Route)
			c.HandleRoute(watch.Added, route)
			return
		},
		UpdateFunc: func(old, obj interface{}) {
			route := obj.(*routeapi.Route)
			c.HandleRoute(watch.Modified, route)
			return
		},
		DeleteFunc: func(obj interface{}) {
			route, ok := obj.(*routeapi.Route)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					glog.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				route, ok = tombstone.Obj.(*routeapi.Route)
				if !ok {
					glog.Errorf("tombstone contained object that is not a route %#v", obj)
					return
				}
			}
			c.HandleRoute(watch.Deleted, route)
			return
		},
	})
	if c.WatchNodes {
		c.StartNodeWatch(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				node := obj.(*kapi.Node)
				c.HandleNode(watch.Added, node)
				return
			},
			UpdateFunc: func(old, obj interface{}) {
				node := obj.(*kapi.Node)
				c.HandleNode(watch.Modified, node)
				return
			},
			DeleteFunc: func(obj interface{}) {
				node, ok := obj.(*kapi.Node)
				if !ok {
					tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						glog.Errorf("couldn't get object from tombstone %+v", obj)
						return
					}
					node, ok = tombstone.Obj.(*kapi.Node)
					if !ok {
						glog.Errorf("tombstone contained object that is not a node %#v", obj)
						return
					}
				}
				c.HandleNode(watch.Deleted, node)
				return
			},
		})
	}
	if c.EnableIngress {
		c.StartIngressWatch(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ingress := obj.(*extensions.Ingress)
				c.HandleIngress(watch.Added, ingress)
				return
			},
			UpdateFunc: func(old, obj interface{}) {
				ingress := obj.(*extensions.Ingress)
				c.HandleIngress(watch.Modified, ingress)
				return
			},
			DeleteFunc: func(obj interface{}) {
				ingress, ok := obj.(*extensions.Ingress)
				if !ok {
					tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						glog.Errorf("couldn't get object from tombstone %+v", obj)
						return
					}
					ingress, ok = tombstone.Obj.(*extensions.Ingress)
					if !ok {
						glog.Errorf("tombstone contained object that is not a ingress %#v", obj)
						return
					}
				}
				c.HandleIngress(watch.Deleted, ingress)
				return
			},
		})
		c.StartSecretsWatch(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				secret := obj.(*kapi.Secret)
				c.HandleSecret(watch.Added, secret)
				return
			},
			UpdateFunc: func(old, obj interface{}) {
				secret := obj.(*kapi.Secret)
				c.HandleSecret(watch.Modified, secret)
				return
			},
			DeleteFunc: func(obj interface{}) {
				secret, ok := obj.(*kapi.Secret)
				if !ok {
					tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						glog.Errorf("couldn't get object from tombstone %+v", obj)
						return
					}
					secret, ok = tombstone.Obj.(*kapi.Secret)
					if !ok {
						glog.Errorf("tombstone contained object that is not a secret %#v", obj)
						return
					}
				}
				c.HandleSecret(watch.Deleted, secret)
				return
			},
		})
	}
	c.StartEndpointsWatch(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			endpoints := obj.(*kapi.Endpoints)
			c.HandleEndpoints(watch.Added, endpoints)
			return
		},
		UpdateFunc: func(old, obj interface{}) {
			endpoints := obj.(*kapi.Endpoints)
			c.HandleEndpoints(watch.Modified, endpoints)
			return
		},
		DeleteFunc: func(obj interface{}) {
			endpoints, ok := obj.(*kapi.Endpoints)
			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					glog.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				endpoints, ok = tombstone.Obj.(*kapi.Endpoints)
				if !ok {
					glog.Errorf("tombstone contained object that is not endpoints %#v", obj)
					return
				}
			}
			c.HandleEndpoints(watch.Deleted, endpoints)
			return
		},
	})

	c.commit()
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
func (c *RouterController) HandleNode(eventType watch.EventType, node *kapi.Node) {
	c.lock.Lock()
	defer c.lock.Unlock()

	glog.V(4).Infof("Processing Node : %s", node.Name)
	glog.V(4).Infof("           Event: %s", eventType)

	if err := c.Plugin.HandleNode(eventType, node); err != nil {
		utilruntime.HandleError(err)
	}
}

// HandleRoute handles a single Route event and synchronizes the router backend.
func (c *RouterController) HandleRoute(eventType watch.EventType, route *routeapi.Route) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.processRoute(eventType, route)
	c.commit()
}

// HandleEndpoints handles a single Endpoints event and refreshes the router backend.
func (c *RouterController) HandleEndpoints(eventType watch.EventType, endpoints *kapi.Endpoints) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.Plugin.HandleEndpoints(eventType, endpoints); err != nil {
		utilruntime.HandleError(err)
	}
	c.commit()
}

// HandleIngress handles a single Ingress event and synchronizes the router backend.
func (c *RouterController) HandleIngress(eventType watch.EventType, ingress *extensions.Ingress) {
	// The ingress translator synchronizes access to its cache with a
	// lock, so calls to it are made outside of the controller lock to
	// avoid unintended interaction.
	events := c.IngressTranslator.TranslateIngressEvent(eventType, ingress)

	c.lock.Lock()
	defer c.lock.Unlock()

	c.processIngressEvents(events)
	c.commit()
}

// HandleSecret handles a single Secret event and synchronizes the router backend.
func (c *RouterController) HandleSecret(eventType watch.EventType, secret *kapi.Secret) {
	// The ingress translator synchronizes access to its cache with a
	// lock, so calls to it are made outside of the controller lock to
	// avoid unintended interaction.
	events := c.IngressTranslator.TranslateSecretEvent(eventType, secret)

	c.lock.Lock()
	defer c.lock.Unlock()

	c.processIngressEvents(events)
	c.commit()
}

// commit notifies the plugin that it is safe to commit state.
func (c *RouterController) commit() {
	syncing := !(c.HasSynced() && (c.Namespaces == nil || c.filteredByNamespace))
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
