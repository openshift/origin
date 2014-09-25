package test

import (
	"errors"
	
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

type RouteRegistry struct {
	Routes         *routeapi.RouteList
}

func NewRouteRegistry() *RouteRegistry {
	return &RouteRegistry{}
}

func (r *RouteRegistry) ListRoutes(labels labels.Selector) (*routeapi.RouteList, error) {
	return r.Routes, nil
}

func (r *RouteRegistry) GetRoute(id string) (*routeapi.Route, error) {
	if r.Routes != nil {
		for _, route := range r.Routes.Items {
			if route.ID == id {
				return &route, nil
			}
		}
	}
	return nil, errors.New("Route " + id + " not found")
}

func (r *RouteRegistry) CreateRoute(route *routeapi.Route) error {
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

func (r *RouteRegistry) UpdateRoute(route *routeapi.Route) error {
	if r.Routes == nil {
		r.Routes = &routeapi.RouteList{}
	}
	newList := []routeapi.Route{}
	found := false
	for _, curRoute := range r.Routes.Items {
		if curRoute.ID == route.ID {
			// route to be updated exists
			found = true
		} else {
			newList = append(newList, curRoute)
		}
	}
	if !found {
		return errors.New("Route " + route.ID + " not found")
	}
	newList = append(newList, *route)
	r.Routes.Items = newList
	return nil
}

func (r *RouteRegistry) DeleteRoute(id string) error {
	if r.Routes == nil {
		r.Routes = &routeapi.RouteList{}
	}
	newList := []routeapi.Route{}
	for _, curRoute := range r.Routes.Items {
		if curRoute.ID != id {
			newList = append(newList, curRoute)
		}
	}
	r.Routes.Items = newList
	return nil
}

func (r *RouteRegistry) WatchRoutes(labels, fields labels.Selector, resourceVersion uint64) (watch.Interface, error) {
	return nil, nil
}
