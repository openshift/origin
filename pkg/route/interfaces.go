package route

import (
	api "github.com/openshift/origin/pkg/route/api"
)

// AllocationPlugin is the interface the route controller dispatches
// requests for RouterShard allocation and name generation.
type AllocationPlugin interface {
	Allocate(*api.Route) (*api.RouterShard, error)
	GenerateHostname(*api.Route, *api.RouterShard) string
}
