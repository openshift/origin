package controller

import (
	"sync"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
)

// RouterController abstracts the details of watching the Route and Endpoints
// resources from the Plugin implementation being used.
type RouterController struct {
	lock          sync.Mutex
	Plugin        router.Plugin
	NextRoute     func() (watch.EventType, *routeapi.Route, error)
	NextEndpoints func() (watch.EventType, *kapi.Endpoints, error)
}

// Run begins watching and syncing.
func (c *RouterController) Run() {
	glog.V(4).Info("Running router controller")
	go util.Forever(c.HandleRoute, 0)
	go util.Forever(c.HandleEndpoints, 0)
}

// HandleRoute handles a single Route event and synchronizes the router backend.
func (c *RouterController) HandleRoute() {
	eventType, route, err := c.NextRoute()
	if err != nil {
		glog.Errorf("Unable to read routes: %v", err)
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	glog.V(4).Infof("Processing Route: %s", route.ServiceName)
	glog.V(4).Infof("           Alias: %s", route.Host)
	glog.V(4).Infof("           Event: %s", eventType)

	c.Plugin.HandleRoute(eventType, route)
}

// HandleEndpoints handles a single Endpoints event and refreshes the router backend.
func (c *RouterController) HandleEndpoints() {
	eventType, endpoints, err := c.NextEndpoints()
	if err != nil {
		glog.Errorf("Unable to read endpoints: %v", err)
		return
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.Plugin.HandleEndpoints(eventType, endpoints)
}
