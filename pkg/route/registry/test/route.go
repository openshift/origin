package test

import (
	"errors"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// RouteRegistry provides an in-memory implementation of
// the route.Registry interface.
type RouteRegistry struct {
	Routes *routeapi.RouteList
}

// NewRouteRegistry creates a new RouteRegistry.
func NewRouteRegistry() *RouteRegistry {
	return &RouteRegistry{}
}

func (r *RouteRegistry) ListRoutes(ctx kapi.Context, labels labels.Selector) (*routeapi.RouteList, error) {
	return r.Routes, nil
}

func (r *RouteRegistry) GetRoute(ctx kapi.Context, id string) (*routeapi.Route, error) {
	if r.Routes != nil {
		for _, route := range r.Routes.Items {
			if route.Name == id {
				return &route, nil
			}
		}
	}
	return nil, errors.New("Route " + id + " not found")
}

func (r *RouteRegistry) CreateRoute(ctx kapi.Context, route *routeapi.Route) error {
	if r.Routes == nil {
		r.Routes = &routeapi.RouteList{}
	}
	newList := []routeapi.Route{}
	for _, curRoute := range r.Routes.Items {
		newList = append(newList, curRoute)
	}
	newList = append(newList, *route)
	r.Routes.Items = newList
	return nil
}

func (r *RouteRegistry) UpdateRoute(ctx kapi.Context, route *routeapi.Route) error {
	if r.Routes == nil {
		r.Routes = &routeapi.RouteList{}
	}
	newList := []routeapi.Route{}
	found := false
	for _, curRoute := range r.Routes.Items {
		if curRoute.Name == route.Name {
			// route to be updated exists
			found = true
		} else {
			newList = append(newList, curRoute)
		}
	}
	if !found {
		return errors.New("Route " + route.Name + " not found")
	}
	newList = append(newList, *route)
	r.Routes.Items = newList
	return nil
}

func (r *RouteRegistry) DeleteRoute(ctx kapi.Context, id string) error {
	if r.Routes == nil {
		r.Routes = &routeapi.RouteList{}
	}
	newList := []routeapi.Route{}
	for _, curRoute := range r.Routes.Items {
		if curRoute.Name != id {
			newList = append(newList, curRoute)
		}
	}
	r.Routes.Items = newList
	return nil
}

func (r *RouteRegistry) WatchRoutes(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, nil
}
