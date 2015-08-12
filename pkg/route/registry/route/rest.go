package route

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/route"
	"github.com/openshift/origin/pkg/route/api"
	"github.com/openshift/origin/pkg/route/api/validation"
)

// HostGeneratedAnnotationKey is the key for an annotation set to "true" if the route's host was generated
const HostGeneratedAnnotationKey = "openshift.io/host.generated"

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	registry  Registry
	allocator route.RouteAllocator
}

// NewREST returns a RESTStorage object that will work against routes.
func NewREST(registry Registry, allocator route.RouteAllocator) *REST {
	return &REST{
		registry:  registry,
		allocator: allocator,
	}
}

// New returns a new Route
func (rs *REST) New() runtime.Object {
	return &api.Route{}
}

// NewList returns a new list of Routes
func (*REST) NewList() runtime.Object {
	return &api.Route{}
}

// List obtains a list of Routes that match label.
func (rs *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	list, err := rs.registry.ListRoutes(ctx, label)
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
func (rs *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	_, err := rs.registry.GetRoute(ctx, id)
	if err != nil {
		return nil, err
	}
	return &kapi.Status{Status: kapi.StatusSuccess}, rs.registry.DeleteRoute(ctx, id)
}

// Create registers a given new Route instance to rs.registry.
func (rs *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	route, ok := obj.(*api.Route)
	if !ok {
		return nil, errors.NewBadRequest(fmt.Sprintf("not a route: %#v", obj))
	}
	if !kapi.ValidNamespace(ctx, &route.ObjectMeta) {
		return nil, errors.NewConflict("route", route.Namespace, fmt.Errorf("Route.Namespace does not match the provided context"))
	}

	shard, err := rs.allocator.AllocateRouterShard(route)
	if err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("allocation error: %s for route: %#v", err, obj))
	}

	if route.Annotations == nil {
		route.Annotations = map[string]string{}
	}
	if len(route.Host) == 0 {
		route.Host = rs.allocator.GenerateHostname(route, shard)
		route.Annotations[HostGeneratedAnnotationKey] = "true"
	} else {
		route.Annotations[HostGeneratedAnnotationKey] = "false"
	}

	if errs := validation.ValidateRoute(route); len(errs) > 0 {
		return nil, errors.NewInvalid("route", route.Name, errs)
	}
	if len(route.Name) == 0 {
		route.Name = uuid.NewUUID().String()
	}

	kapi.FillObjectMetaSystemFields(ctx, &route.ObjectMeta)

	err = rs.registry.CreateRoute(ctx, route)
	if err != nil {
		return nil, err
	}
	return rs.registry.GetRoute(ctx, route.Name)
}

// Update replaces a given Route instance with an existing instance in rs.registry.
func (rs *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	route, ok := obj.(*api.Route)
	if !ok {
		return nil, false, errors.NewBadRequest(fmt.Sprintf("not a route: %#v", obj))
	}
	if !kapi.ValidNamespace(ctx, &route.ObjectMeta) {
		return nil, false, errors.NewConflict("route", route.Namespace, fmt.Errorf("Route.Namespace does not match the provided context"))
	}

	old, err := rs.Get(ctx, route.Name)
	if err != nil {
		return nil, false, err
	}
	if errs := validation.ValidateRouteUpdate(route, old.(*api.Route)); len(errs) > 0 {
		return nil, false, errors.NewInvalid("route", route.Name, errs)
	}

	// TODO: Convert to generic etcd
	// TODO: Call ValidateRouteUpdate->ValidateObjectMetaUpdate
	// TODO: In the UpdateStrategy.PrepareForUpdate, set the HostGeneratedAnnotationKey annotation to "false" if the updated route object modifies the host

	err = rs.registry.UpdateRoute(ctx, route)
	if err != nil {
		return nil, false, err
	}
	out, err := rs.registry.GetRoute(ctx, route.Name)
	return out, false, err
}

// Watch returns Routes events via a watch.Interface.
// It implements apiserver.ResourceWatcher.
func (rs *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return rs.registry.WatchRoutes(ctx, label, field, resourceVersion)
}
