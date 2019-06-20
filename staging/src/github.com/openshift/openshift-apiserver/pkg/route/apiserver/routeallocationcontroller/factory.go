package routeallocationcontroller

import (
	"github.com/openshift/openshift-apiserver/pkg/route/apiserver/routeinterfaces"
)

// RouteAllocationControllerFactory creates a RouteAllocationController
// that allocates router shards to specific routes.
type RouteAllocationControllerFactory struct {
}

// Create a RouteAllocationController instance.
func (factory *RouteAllocationControllerFactory) Create(plugin routeinterfaces.AllocationPlugin) *RouteAllocationController {
	return &RouteAllocationController{Plugin: plugin}
}
