package route

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

// Registry is an interface for things that know how to store Routes.
type Registry interface {
	// ListRoutes obtains list of routes that match a selector.
	ListRoutes(ctx apirequest.Context, options *metainternal.ListOptions) (*routeapi.RouteList, error)
	// GetRoute retrieves a specific route.
	GetRoute(ctx apirequest.Context, routeID string, options *metav1.GetOptions) (*routeapi.Route, error)
	// CreateRoute creates a new route.
	CreateRoute(ctx apirequest.Context, route *routeapi.Route) error
	// UpdateRoute updates a route.
	UpdateRoute(ctx apirequest.Context, route *routeapi.Route) error
	// DeleteRoute deletes a route.
	DeleteRoute(ctx apirequest.Context, routeID string) error
	// WatchRoutes watches for new/modified/deleted routes.
	WatchRoutes(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error)
}
