package allocation

import (
	"github.com/openshift/origin/pkg/route"
)

// RouteAllocationControllerFactory creates a RouteAllocationController
// that allocates router shards to specific routes.
type RouteAllocationControllerFactory struct {
}

// Create a RouteAllocationController instance.
func (factory *RouteAllocationControllerFactory) Create(plugin route.AllocationPlugin) *RouteAllocationController {
	return &RouteAllocationController{Plugin: plugin}
}
