package etcd

import (
	"fmt"

	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/route/api"
)

// Etcd implements route.Registry backed by etcd.
type Etcd struct {
	tools.EtcdHelper
}

// New creates an etcd registry.
func New(helper tools.EtcdHelper) *Etcd {
	return &Etcd{
		EtcdHelper: helper,
	}
}

func makeRouteKey(id string) string {
	return "/routes/" + id
}

// ListRoutes obtains a list of Routes.
func (registry *Etcd) ListRoutes(selector labels.Selector) (*api.RouteList, error) {
	allRoutes := api.RouteList{}
	err := registry.ExtractList("/routes", &allRoutes.Items, &allRoutes.ResourceVersion)
	if err != nil {
		return nil, err
	}
	filtered := []api.Route{}
	for _, route := range allRoutes.Items {
		if selector.Matches(labels.Set(route.Labels)) {
			filtered = append(filtered, route)
		}
	}
	allRoutes.Items = filtered
	return &allRoutes, nil

}

// GetRoute gets a specific Route specified by its ID.
func (registry *Etcd) GetRoute(routeID string) (*api.Route, error) {
	route := api.Route{}
	err := registry.ExtractObj(makeRouteKey(routeID), &route, false)
	if err != nil {
		return nil, etcderr.InterpretGetError(err, "route", routeID)
	}
	return &route, nil
}

// CreateRoute creates a new Route.
func (registry *Etcd) CreateRoute(route *api.Route) error {
	err := registry.CreateObj(makeRouteKey(route.ID), route, 0)
	return etcderr.InterpretCreateError(err, "route", route.ID)
}

// UpdateRoute replaces an existing Route.
func (registry *Etcd) UpdateRoute(route *api.Route) error {
	err := registry.SetObj(makeRouteKey(route.ID), route)
	return etcderr.InterpretUpdateError(err, "route", route.ID)
}

// DeleteRoute deletes a Route specified by its ID.
func (registry *Etcd) DeleteRoute(routeID string) error {
	key := makeRouteKey(routeID)
	err := registry.Delete(key, true)
	return etcderr.InterpretDeleteError(err, "route", routeID)
}

// WatchRoutes begins watching for new, changed, or deleted route configurations.
func (registry *Etcd) WatchRoutes(label, field labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	if !label.Empty() {
		return nil, fmt.Errorf("label selectors are not supported on routes yet")
	}
	if value, found := field.RequiresExactMatch("ID"); found {
		return registry.Watch(makeRouteKey(value), resourceVersion), nil
	}
	if field.Empty() {
		return registry.WatchList("/routes", resourceVersion, tools.Everything)
	}
	return nil, fmt.Errorf("only the 'ID' and default (everything) field selectors are supported")
}
