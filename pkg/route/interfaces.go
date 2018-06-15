package route

import (
	api "github.com/openshift/origin/pkg/route/apis/route"
)

// AllocationPlugin is the interface the route controller dispatches
// requests for RouterShard allocation and name generation.
type AllocationPlugin interface {
	Allocate(*api.Route) (*api.RouterShard, error)
	GenerateHostname(*api.Route, *api.RouterShard) string
}

// RouteAllocator is the interface for the route allocation controller
// which handles requests for RouterShard allocation and name generation.
type RouteAllocator interface {
	AllocateRouterShard(*api.Route) (*api.RouterShard, error)
	GenerateHostname(*api.Route, *api.RouterShard) string
}
