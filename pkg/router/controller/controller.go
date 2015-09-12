package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
)

// NamespaceLister returns all the names that should be watched by the client
type NamespaceLister interface {
	NamespaceNames() (util.StringSet, error)
}

// RouterController abstracts the details of watching the Route and Endpoints
// resources from the Plugin implementation being used.
type RouterController struct {
	lock sync.Mutex

	Plugin        router.Plugin
	NextRoute     func() (watch.EventType, *routeapi.Route, error)
	NextEndpoints func() (watch.EventType, *kapi.Endpoints, error)

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
		go util.Forever(c.HandleNamespaces, c.NamespaceSyncInterval)
	}
	go util.Forever(c.HandleRoute, 0)
	go util.Forever(c.HandleEndpoints, 0)
}

func (c *RouterController) HandleNamespaces() {
	for i := 0; i < c.NamespaceRetries; i++ {
		namespaces, err := c.Namespaces.NamespaceNames()
		if err == nil {
			c.lock.Lock()
			defer c.lock.Unlock()

			glog.V(4).Infof("Updating watched namespaces: %v", namespaces)
			if err := c.Plugin.HandleNamespaces(namespaces); err != nil {
				util.HandleError(err)
			}
			return
		}
		util.HandleError(fmt.Errorf("unable to find namespaces for router: %v", err))
		time.Sleep(c.NamespaceWaitInterval)
	}
	glog.V(4).Infof("Unable to update list of namespaces")
}

// HandleRoute handles a single Route event and synchronizes the router backend.
func (c *RouterController) HandleRoute() {
	eventType, route, err := c.NextRoute()
	if err != nil {
		util.HandleError(fmt.Errorf("unable to read routes: %v", err))
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	glog.V(4).Infof("Processing Route: %s", route.Spec.To.Name)
	glog.V(4).Infof("           Alias: %s", route.Spec.Host)
	glog.V(4).Infof("           Event: %s", eventType)

	if err := c.Plugin.HandleRoute(eventType, route); err != nil {
		util.HandleError(err)
	}
}

// HandleEndpoints handles a single Endpoints event and refreshes the router backend.
func (c *RouterController) HandleEndpoints() {
	eventType, endpoints, err := c.NextEndpoints()
	if err != nil {
		util.HandleError(fmt.Errorf("unable to read endpoints: %v", err))
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.Plugin.HandleEndpoints(eventType, endpoints); err != nil {
		util.HandleError(err)
	}
}
