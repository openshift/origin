package etcd

import (
	"fmt"

	etcderr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors/etcd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"strconv"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeetcd "github.com/GoogleCloudPlatform/kubernetes/pkg/registry/etcd"
	"github.com/openshift/origin/pkg/route/api"
)

const (
	// RoutePath is the path to route image in etcd
	RoutePath string = "/routes"
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

func makeRouteListKey(ctx kapi.Context) string {
	return kubeetcd.MakeEtcdListKey(ctx, RoutePath)
}

func makeRouteKey(ctx kapi.Context, id string) (string, error) {
	return kubeetcd.MakeEtcdItemKey(ctx, RoutePath, id)
}

// ListRoutes obtains a list of Routes.
func (registry *Etcd) ListRoutes(ctx kapi.Context, selector labels.Selector) (*api.RouteList, error) {
	allRoutes := api.RouteList{}
	err := registry.ExtractToList(makeRouteListKey(ctx), &allRoutes)
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
	err = registry.ExtractObj(key, &route, false)
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
	err = registry.CreateObj(key, route, 0)
	return etcderr.InterpretCreateError(err, "route", route.Name)
}

// UpdateRoute replaces an existing Route.
func (registry *Etcd) UpdateRoute(ctx kapi.Context, route *api.Route) error {
	key, err := makeRouteKey(ctx, route.Name)
	if err != nil {
		return err
	}
	err = registry.SetObj(key, route)
	return etcderr.InterpretUpdateError(err, "route", route.Name)
}

// DeleteRoute deletes a Route specified by its ID.
func (registry *Etcd) DeleteRoute(ctx kapi.Context, routeID string) error {
	key, err := makeRouteKey(ctx, routeID)
	if err != nil {
		return err
	}
	err = registry.Delete(key, true)
	return etcderr.InterpretDeleteError(err, "route", routeID)
}

// WatchRoutes begins watching for new, changed, or deleted route configurations.
func (registry *Etcd) WatchRoutes(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	if !label.Empty() {
		return nil, fmt.Errorf("label selectors are not supported on routes yet")
	}

	version, err := parseWatchResourceVersion(resourceVersion, "pod")
	if err != nil {
		return nil, err
	}

	if value, found := field.RequiresExactMatch("ID"); found {
		key, err := makeRouteKey(ctx, value)
		if err != nil {
			return nil, err
		}
		return registry.Watch(key, version), nil
	}

	if field.Empty() {
		key := kubeetcd.MakeEtcdListKey(ctx, RoutePath)
		return registry.WatchList(key, version, tools.Everything)
	}
	return nil, fmt.Errorf("only the 'ID' and default (everything) field selectors are supported")
}

// parseWatchResourceVersion takes a resource version argument and converts it to
// the etcd version we should pass to helper.Watch(). Because resourceVersion is
// an opaque value, the default watch behavior for non-zero watch is to watch
// the next value (if you pass "1", you will see updates from "2" onwards).
func parseWatchResourceVersion(resourceVersion, kind string) (uint64, error) {
	if resourceVersion == "" || resourceVersion == "0" {
		return 0, nil
	}
	version, err := strconv.ParseUint(resourceVersion, 10, 64)
	if err != nil {
		return 0, etcderr.InterpretResourceVersionError(err, kind, resourceVersion)
	}
	return version + 1, nil
}
