package route

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/route/api"
)

// Registry is an interface for things that know how to store Routes.
type Registry interface {
	// ListRoutes obtains list of routes that match a selector.
	ListRoutes(selector labels.Selector) (*api.RouteList, error)
	// GetRoute retrieves a specific route.
	GetRoute(routeID string) (*api.Route, error)
	// CreateRoute creates a new route.
	CreateRoute(route *api.Route) error
	// UpdateRoute updates a route.
	UpdateRoute(route *api.Route) error
	// DeleteRoute deletes a route.
	DeleteRoute(routeID string) error
	// WatchRoutes watches for new/modified/deleted routes.
	WatchRoutes(labels, fields labels.Selector, resourceVersion uint64) (watch.Interface, error)
}
