package etcd

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kubeetcd "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/storage"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/route/api"
)

const (
	// RoutePath is the path to route image in etcd
	RoutePath string = "/routes"
)

// Etcd implements route.Registry backed by etcd.
type Etcd struct {
	storage.Interface
}

// New creates an etcd registry.
func New(storage storage.Interface) *Etcd {
	return &Etcd{
		storage,
	}
}

func makeRouteListKey(ctx kapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, RoutePath)
}

func makeRouteKey(ctx kapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, RoutePath, id)
}

// ListRoutes obtains a list of Routes.
func (registry *Etcd) ListRoutes(ctx kapi.Context, selector labels.Selector) (*api.RouteList, error) {
	allRoutes := api.RouteList{}
	err := registry.List(makeRouteListKey(ctx), &allRoutes)
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
func (registry *Etcd) GetRoute(ctx kapi.Context, routeID string) (*api.Route, error) {
	route := api.Route{}
	key, err := makeRouteKey(ctx, routeID)
	if err != nil {
		return nil, err
	}
	err = registry.Get(key, &route, false)
	if err != nil {
		return nil, etcderr.InterpretGetError(err, "route", routeID)
	}
	return &route, nil
}

// CreateRoute creates a new Route.
func (registry *Etcd) CreateRoute(ctx kapi.Context, route *api.Route) error {
	key, err := makeRouteKey(ctx, route.Name)
	if err != nil {
		return err
	}
	err = registry.Create(key, route, nil, 0)
	return etcderr.InterpretCreateError(err, "route", route.Name)
}

// UpdateRoute replaces an existing Route.
func (registry *Etcd) UpdateRoute(ctx kapi.Context, route *api.Route) error {
	key, err := makeRouteKey(ctx, route.Name)
	if err != nil {
		return err
	}
	err = registry.Set(key, route, nil, 0)
	return etcderr.InterpretUpdateError(err, "route", route.Name)
}

// DeleteRoute deletes a Route specified by its ID.
func (registry *Etcd) DeleteRoute(ctx kapi.Context, routeID string) error {
	key, err := makeRouteKey(ctx, routeID)
	if err != nil {
		return err
	}
	err = registry.Delete(key, &api.Route{})
	return etcderr.InterpretDeleteError(err, "route", routeID)
}

// WatchRoutes begins watching for new, changed, or deleted route configurations.
func (registry *Etcd) WatchRoutes(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	if !label.Empty() {
		return nil, fmt.Errorf("label selectors are not supported on routes yet")
	}

	version, err := storage.ParseWatchResourceVersion(resourceVersion, "pod")
	if err != nil {
		return nil, err
	}

	if value, found := field.RequiresExactMatch("ID"); found {
		key, err := makeRouteKey(ctx, value)
		if err != nil {
			return nil, err
		}
		return registry.Watch(key, version, storage.Everything)
	}

	if field.Empty() {
		key := kubeetcd.MakeEtcdListKey(ctx, RoutePath)
		return registry.WatchList(key, version, storage.Everything)
	}
	return nil, fmt.Errorf("only the 'ID' and default (everything) field selectors are supported")
}
