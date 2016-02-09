package route

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/route/api"
)

// Registry is an interface for things that know how to store Routes.
type Registry interface {
	// ListRoutes obtains list of routes that match a selector.
	ListRoutes(ctx kapi.Context, options *kapi.ListOptions) (*api.RouteList, error)
	// GetRoute retrieves a specific route.
	GetRoute(ctx kapi.Context, routeID string) (*api.Route, error)
	// CreateRoute creates a new route.
	CreateRoute(ctx kapi.Context, route *api.Route) error
	// UpdateRoute updates a route.
	UpdateRoute(ctx kapi.Context, route *api.Route) error
	// DeleteRoute deletes a route.
	DeleteRoute(ctx kapi.Context, routeID string) error
	// WatchRoutes watches for new/modified/deleted routes.
	WatchRoutes(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error)
}
