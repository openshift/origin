package route

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/route/api/validation"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	registry Registry
}

func NewREST(registry Registry) *REST {
	return &REST{
		registry: registry,
	}
}

func (rs *REST) New() runtime.Object {
	return &api.Route{}
}

// List obtains a list of Routes that match selector.
func (rs *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	list, err := rs.registry.ListRoutes(ctx, selector)
	if err != nil {
		return nil, err
	}
	return list, err
}

// Get obtains the route specified by its id.
func (rs *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	route, err := rs.registry.GetRoute(ctx, id)
	if err != nil {
		return nil, err
	}
	return route, err
}

// Delete asynchronously deletes the Route specified by its id.
func (rs *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	_, err := rs.registry.GetRoute(ctx, id)
	if err != nil {
		return nil, err
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kapi.Status{Status: kapi.StatusSuccess}, rs.registry.DeleteRoute(ctx, id)
	}), nil
}

// Create registers a given new Route instance to rs.registry.
func (rs *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	route, ok := obj.(*api.Route)
	if !ok {
		return nil, fmt.Errorf("not a route: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &route.ObjectMeta) {
		return nil, errors.NewConflict("route", route.Namespace, fmt.Errorf("Route.Namespace does not match the provided context"))
	}

	if errs := validation.ValidateRoute(route); len(errs) > 0 {
		return nil, errors.NewInvalid("route", route.Name, errs)
	}
	if len(route.Name) == 0 {
		route.Name = uuid.NewUUID().String()
	}

	route.CreationTimestamp = util.Now()

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := rs.registry.CreateRoute(ctx, route)
		if err != nil {
			return nil, err
		}
		return rs.registry.GetRoute(ctx, route.Name)
	}), nil
}

// Update replaces a given Route instance with an existing instance in rs.registry.
func (rs *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	route, ok := obj.(*api.Route)
	if !ok {
		return nil, fmt.Errorf("not a route: %#v", obj)
	}
	if len(route.Name) == 0 {
		return nil, fmt.Errorf("name is unspecified: %#v", route)
	}
	if !kapi.ValidNamespace(ctx, &route.ObjectMeta) {
		return nil, errors.NewConflict("route", route.Namespace, fmt.Errorf("Route.Namespace does not match the provided context"))
	}

	if errs := validation.ValidateRoute(route); len(errs) > 0 {
		return nil, errors.NewInvalid("route", route.Name, errs)
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := rs.registry.UpdateRoute(ctx, route)
		if err != nil {
			return nil, err
		}
		return rs.registry.GetRoute(ctx, route.Name)
	}), nil
}

// Watch returns Routes events via a watch.Interface.
// It implements apiserver.ResourceWatcher.
func (rs *REST) Watch(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return rs.registry.WatchRoutes(ctx, label, field, resourceVersion)
}
