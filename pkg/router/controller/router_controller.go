package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routev1 "github.com/openshift/api/route/v1"
	projectclient "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	"github.com/openshift/origin/pkg/router"
)

// RouterController abstracts the details of watching resources like Routes, Endpoints, etc.
// used by the plugin implementation.
type RouterController struct {
	lock sync.Mutex

	Plugin router.Plugin

	firstSyncDone bool

	FilteredNamespaceNames sets.String
	NamespaceLabels        labels.Selector
	// Holds Namespace --> RouteName --> RouteObject
	NamespaceRoutes map[string]map[string]*routev1.Route
	// Holds Namespace --> EndpointsName --> EndpointsObject
	NamespaceEndpoints map[string]map[string]*kapi.Endpoints

	ProjectClient       projectclient.ProjectInterface
	ProjectLabels       labels.Selector
	ProjectSyncInterval time.Duration
	ProjectWaitInterval time.Duration
	ProjectRetries      int

	WatchNodes bool
}

// Run begins watching and syncing.
func (c *RouterController) Run() {
	glog.V(4).Info("Running router controller")
	if c.ProjectLabels != nil {
		c.HandleProjects()
		go utilwait.Forever(c.HandleProjects, c.ProjectSyncInterval)
	}
	c.handleFirstSync()
}

func (c *RouterController) HandleProjects() {
	for i := 0; i < c.ProjectRetries; i++ {
		names, err := c.GetFilteredProjectNames()
		if err == nil {
			// Return early if there is no new change
			if names.Equal(c.FilteredNamespaceNames) {
				return
			}
			c.lock.Lock()
			defer c.lock.Unlock()

			c.FilteredNamespaceNames = names
			c.UpdateNamespaces()
			c.Commit()
			return
		}
		utilruntime.HandleError(fmt.Errorf("unable to get filtered projects for router: %v", err))
		time.Sleep(c.ProjectWaitInterval)
	}
	glog.V(4).Infof("Unable to update list of filtered projects")
}

func (c *RouterController) GetFilteredProjectNames() (sets.String, error) {
	names := sets.String{}
	all, err := c.ProjectClient.List(v1.ListOptions{LabelSelector: c.ProjectLabels.String()})
	if err != nil {
		return nil, err
	}
	for _, item := range all.Items {
		names.Insert(item.Name)
	}
	return names, nil
}

func (c *RouterController) processNamespace(eventType watch.EventType, ns *kapi.Namespace) {
	before := c.FilteredNamespaceNames.Has(ns.Name)
	switch eventType {
	case watch.Added, watch.Modified:
		if c.NamespaceLabels.Matches(labels.Set(ns.Labels)) {
			c.FilteredNamespaceNames.Insert(ns.Name)
		} else {
			c.FilteredNamespaceNames.Delete(ns.Name)
		}
	case watch.Deleted:
		c.FilteredNamespaceNames.Delete(ns.Name)
	}
	after := c.FilteredNamespaceNames.Has(ns.Name)

	// Namespace added or deleted
	if (!before && after) || (before && !after) {
		glog.V(5).Infof("Processing matched namespace: %s with labels: %v", ns.Name, ns.Labels)

		c.UpdateNamespaces()

		// New namespace created or router matching labels added to existing namespace
		// Routes for new namespace will be handled by HandleRoute().
		// For existing namespace, add corresponding endpoints/routes as watch endpoints
		// and routes won't be updated till the next resync interval which could be few mins.
		if !before && after {
			if epMap, ok := c.NamespaceEndpoints[ns.Name]; ok {
				for _, ep := range epMap {
					if err := c.Plugin.HandleEndpoints(watch.Modified, ep); err != nil {
						utilruntime.HandleError(err)
					}
				}
			}

			if routeMap, ok := c.NamespaceRoutes[ns.Name]; ok {
				for _, route := range routeMap {
					c.processRoute(watch.Modified, route)
				}
			}
		}
	}
}

func (c *RouterController) UpdateNamespaces() {
	// Note: Need to clone the filtered namespace names or else any updates
	//       we make locally in processNamespace() will be immediately
	//       reflected to plugins in the chain beneath us. This creates
	//       cleanup issues as old == new in Plugin.HandleNamespaces().
	namespaces := sets.NewString(c.FilteredNamespaceNames.List()...)

	glog.V(4).Infof("Updating watched namespaces: %v", namespaces)
	if err := c.Plugin.HandleNamespaces(namespaces); err != nil {
		utilruntime.HandleError(err)
	}
}

func (c *RouterController) RecordNamespaceEndpoints(eventType watch.EventType, ep *kapi.Endpoints) {
	switch eventType {
	case watch.Added, watch.Modified:
		if _, ok := c.NamespaceEndpoints[ep.Namespace]; !ok {
			c.NamespaceEndpoints[ep.Namespace] = make(map[string]*kapi.Endpoints)
		}
		c.NamespaceEndpoints[ep.Namespace][ep.Name] = ep
	case watch.Deleted:
		if _, ok := c.NamespaceEndpoints[ep.Namespace]; ok {
			delete(c.NamespaceEndpoints[ep.Namespace], ep.Name)
			if len(c.NamespaceEndpoints[ep.Namespace]) == 0 {
				delete(c.NamespaceEndpoints, ep.Namespace)
			}
		}
	}
}

func (c *RouterController) RecordNamespaceRoutes(eventType watch.EventType, rt *routev1.Route) {
	switch eventType {
	case watch.Added, watch.Modified:
		if _, ok := c.NamespaceRoutes[rt.Namespace]; !ok {
			c.NamespaceRoutes[rt.Namespace] = make(map[string]*routev1.Route)
		}
		c.NamespaceRoutes[rt.Namespace][rt.Name] = rt
	case watch.Deleted:
		if _, ok := c.NamespaceRoutes[rt.Namespace]; ok {
			delete(c.NamespaceRoutes[rt.Namespace], rt.Name)
			if len(c.NamespaceRoutes[rt.Namespace]) == 0 {
				delete(c.NamespaceRoutes, rt.Namespace)
			}
		}
	}
}

func (c *RouterController) HandleNamespace(eventType watch.EventType, obj interface{}) {
	ns := obj.(*kapi.Namespace)
	c.lock.Lock()
	defer c.lock.Unlock()

	glog.V(4).Infof("Processing Namespace: %s", ns.Name)
	glog.V(4).Infof("           Event: %s", eventType)

	c.processNamespace(eventType, ns)
	c.Commit()
}

// HandleNode handles a single Node event and synchronizes the router backend
func (c *RouterController) HandleNode(eventType watch.EventType, obj interface{}) {
	node := obj.(*kapi.Node)
	c.lock.Lock()
	defer c.lock.Unlock()

	glog.V(4).Infof("Processing Node: %s", node.Name)
	glog.V(4).Infof("           Event: %s", eventType)

	if err := c.Plugin.HandleNode(eventType, node); err != nil {
		utilruntime.HandleError(err)
	}
}

// HandleRoute handles a single Route event and synchronizes the router backend.
func (c *RouterController) HandleRoute(eventType watch.EventType, obj interface{}) {
	route := obj.(*routev1.Route)
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

	c.RecordNamespaceEndpoints(eventType, endpoints)
	if err := c.Plugin.HandleEndpoints(eventType, endpoints); err != nil {
		utilruntime.HandleError(err)
	}
	c.Commit()
}

// Commit notifies the plugin that it is safe to commit state.
func (c *RouterController) Commit() {
	if c.firstSyncDone {
		if err := c.Plugin.Commit(); err != nil {
			utilruntime.HandleError(err)
		}
	}
}

// processRoute logs and propagates a route event to the plugin
func (c *RouterController) processRoute(eventType watch.EventType, route *routev1.Route) {
	glog.V(4).Infof("Processing route: %s/%s -> %s %s", route.Namespace, route.Name, route.Spec.To.Name, route.UID)
	glog.V(4).Infof("           Alias: %s", route.Spec.Host)
	if len(route.Spec.Path) > 0 {
		glog.V(4).Infof("           Path: %s", route.Spec.Path)
	}
	glog.V(4).Infof("           Event: %s rv=%s", eventType, route.ResourceVersion)

	c.RecordNamespaceRoutes(eventType, route)
	if err := c.Plugin.HandleRoute(eventType, route); err != nil {
		utilruntime.HandleError(err)
	}
}

func (c *RouterController) handleFirstSync() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.firstSyncDone = true
	glog.V(4).Infof("Router first sync complete")
	c.Commit()
}
