package controller

import (
	"strings"
	"sync"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/golang/glog"

	routeapi "github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/router"
)

// RouterController is responsible synchronizing the router backend state
// with the known Route and Endpoint states.
type RouterController struct {
	lock          sync.Mutex
	Router        router.Router
	NextRoute     func() (watch.EventType, *routeapi.Route)
	NextEndpoints func() (watch.EventType, *kapi.Endpoints)
}

// Run begins watching and syncing.
func (c *RouterController) Run() {
	glog.V(4).Info("Running router controller")
	go util.Forever(c.HandleRoute, 0)
	go util.Forever(c.HandleEndpoints, 0)
}

// HandleRoute handles a single Route event and synchronizes the router backend.
func (c *RouterController) HandleRoute() {
	eventType, route := c.NextRoute()

	c.lock.Lock()
	defer c.lock.Unlock()

	key := routeKey(*route)

	glog.V(4).Infof("Processing Route: %s", route.ServiceName)
	glog.V(4).Infof("           Alias: %s", route.Host)
	glog.V(4).Infof("           Event: %s", eventType)

	if _, ok := c.Router.FindFrontend(key); !ok {
		c.Router.CreateFrontend(key, "")
	}

	switch eventType {
	case watch.Added, watch.Modified:
		glog.V(4).Infof("Modifying routes for %s", key)
		c.Router.AddAlias(route.Host, key)
	case watch.Deleted:
		glog.V(4).Infof("Deleting routes for %s", key)
		c.Router.RemoveAlias(route.Host, key)
	}

	c.Router.WriteConfig()
	c.Router.ReloadRouter()
}

// HandleEndpoints handles a single Endpoints event and refreshes the router backend.
func (c *RouterController) HandleEndpoints() {
	eventType, endpoints := c.NextEndpoints()

	c.lock.Lock()
	defer c.lock.Unlock()

	key := endpointsKey(*endpoints)

	glog.V(4).Infof("Processing %d Endpoints for Name : %s", len(endpoints.Endpoints), endpoints.Name)

	for i, e := range endpoints.Endpoints {
		glog.V(4).Infof("  Endpoint %d : %s", i, e)
	}

	if _, ok := c.Router.FindFrontend(key); !ok {
		c.Router.CreateFrontend(key, "") //"www."+endpoints.ID+".com"
	}

	// Delete the backends and rebuild the new state.
	c.Router.DeleteBackends(key)

	switch eventType {
	case watch.Added, watch.Modified:
		glog.V(4).Infof("Modifying endpoints for %s", key)
		routerEndpoints := make([]router.Endpoint, len(endpoints.Endpoints))

		for i, e := range endpoints.Endpoints {
			ep := router.Endpoint{}
			if strings.Contains(e, ":") {
				eArr := strings.Split(e, ":")
				ep.IP = eArr[0]
				ep.Port = eArr[1]
			} else if e == "" {
				continue
			} else {
				ep.IP = e
				ep.Port = "80"
			}
			routerEndpoints[i] = ep
		}

		c.Router.AddRoute(key, "", "", nil, routerEndpoints)
	}

	c.Router.WriteConfig()
	c.Router.ReloadRouter()
}

// TODO: the internal keys for routes and endpoints should be namespaced.  Currently
// there is an upstream issue where the namespace is not set on non-resolved endpoints.
// A fix has been submitted and we should consume it in the next rebase.

// routeKey returns the internal router key to use for the given Route.
func routeKey(route routeapi.Route) string {
	return route.ServiceName
}

// endpointsKey returns the internal router key to use for the given Endpoints.
func endpointsKey(endpoints kapi.Endpoints) string {
	return endpoints.Name
}
